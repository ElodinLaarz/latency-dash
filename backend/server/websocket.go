package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/elodin/latency-dash/backend/calculator"
	"github.com/elodin/latency-dash/backend/proto"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebSocketServer struct {
	calculator *calculator.MetricsCalculator
	clients    map[*websocket.Conn]bool
	clientsMu  sync.Mutex
}

func NewWebSocketServer(calculator *calculator.MetricsCalculator) *WebSocketServer {
	return &WebSocketServer{
		calculator: calculator,
		clients:    make(map[*websocket.Conn]bool),
	}
}

func (s *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	func() {
		s.clientsMu.Lock()
		defer s.clientsMu.Unlock()
		s.clients[conn] = true
	}()

	// Unregister client when done
	defer func() {
		s.clientsMu.Lock()
		defer s.clientsMu.Unlock()
		delete(s.clients, conn)
	}()

	// Handle incoming messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		// Process subscription
		var msg proto.SubscriptionMessage
		err = conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}
		s.handleSubscription(conn, &msg)
	}
}

func (s *WebSocketServer) handleSubscription(conn *websocket.Conn, msg *proto.SubscriptionMessage) {
	// For now, we'll just log the subscription
	log.Printf("New subscription: %+v", msg)
}

func (s *WebSocketServer) Broadcast(update *proto.MetricsUpdate) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		err := client.WriteJSON(update)
		if err != nil {
			log.Printf("Error sending update to client: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}
