package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/elodin/latency-dash/backend/calculator"
	"github.com/elodin/latency-dash/backend/proto"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow connections from any origin during development
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should validate the origin
		// return r.Header.Get("Origin") == "https://your-production-domain.com"
		return true
	},
}

type WebSocketServer struct {
	calculator *calculator.MetricsCalculator
	clients    map[*websocket.Conn]bool
	clientsMu  sync.Mutex
}

func NewWebSocketServer(calculator *calculator.MetricsCalculator) *WebSocketServer {
	server := &WebSocketServer{
		calculator: calculator,
		clients:    make(map[*websocket.Conn]bool),
	}

	// Start a goroutine to listen for metrics updates
	go func() {
		subscriber := calculator.Subscribe()
		for update := range subscriber {
			server.Broadcast(update)
		}
	}()

	return server
}

func (s *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Register client
	func() {
		s.clientsMu.Lock()
		defer s.clientsMu.Unlock()
		s.clients[conn] = true
		log.Printf("New client connected. Total clients: %d", len(s.clients))
	}()

	// Set up a context to handle client disconnection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to handle ping/pong
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send ping message
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
					log.Printf("Error sending ping: %v", err)
					conn.Close()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Handle incoming messages
	for {
		// Read the raw message
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		// Unmarshal the message
		var wsMsg proto.WebSocketMessage
		if err := protojson.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		switch msg := wsMsg.Content.(type) {
		case *proto.WebSocketMessage_Subscription:
			s.handleSubscription(conn, msg.Subscription)
		default:
			log.Printf("Received unhandled message type: %T", msg)
		}
	}
}

func (s *WebSocketServer) handleSubscription(conn *websocket.Conn, msg *proto.SubscriptionMessage) {
	log.Printf("New subscription: %+v", msg)
	
	// Acknowledge the subscription
	ack := &proto.WebSocketMessage{
		Content: &proto.WebSocketMessage_SubscriptionAck{
			SubscriptionAck: &proto.SubscriptionAck{
				TargetId:        msg.TargetId,
				Keys:            msg.Keys,
				SplitByMetadata: msg.SplitByMetadata,
				Success:         true,
				Message:         "Subscription successful",
			},
		},
	}
	
	// Marshal the message to JSON with camelCase field names
	marshaler := protojson.MarshalOptions{
		UseProtoNames: false, // Use camelCase instead of snake_case
	}
	data, err := marshaler.Marshal(ack)
	if err != nil {
		log.Printf("Error marshaling subscription ack: %v", err)
		return
	}

	// Send the acknowledgment
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending subscription ack: %v", err)
		return
	}
	
	// Send current snapshot of all metrics
	allMetrics := s.calculator.GetAllMetrics()
	log.Printf("Sending snapshot of %d metrics to new subscriber", len(allMetrics))
	
	for _, update := range allMetrics {
		wsMsg := &proto.WebSocketMessage{
			Content: &proto.WebSocketMessage_MetricsUpdate{
				MetricsUpdate: update,
			},
		}
		
		data, err := marshaler.Marshal(wsMsg)
		if err != nil {
			log.Printf("Error marshaling metrics update: %v", err)
			continue
		}
		
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending metrics snapshot: %v", err)
			return
		}
	}
	
	if msg.TargetId != "" {
		log.Printf("Subscribed to target: %s, keys: %v, split by metadata: %v", 
			msg.TargetId, msg.Keys, msg.SplitByMetadata)
	} else {
		log.Printf("Subscribed to all targets, keys: %v, split by metadata: %v", 
			msg.Keys, msg.SplitByMetadata)
	}
}

func (s *WebSocketServer) Broadcast(update *proto.MetricsUpdate) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if len(s.clients) == 0 {
		log.Println("No clients connected to broadcast to")
		return
	}

	// Wrap the MetricsUpdate in a WebSocketMessage envelope
	wsMsg := &proto.WebSocketMessage{
		Content: &proto.WebSocketMessage_MetricsUpdate{
			MetricsUpdate: update,
		},
	}

	// Marshal to JSON with camelCase field names
	marshaler := protojson.MarshalOptions{
		UseProtoNames: false, // Use camelCase instead of snake_case
	}
	data, err := marshaler.Marshal(wsMsg)
	if err != nil {
		log.Printf("Error marshaling metrics update: %v", err)
		return
	}
	
	// Log first message for debugging (only once)
	// Uncomment to debug: log.Printf("Broadcasting metrics update: %s", string(data))

	for client := range s.clients {
		// Set a write deadline to prevent blocking
		err := client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			log.Printf("Error setting write deadline: %v", err)
			continue
		}

		err = client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Printf("Error sending update to client: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}
