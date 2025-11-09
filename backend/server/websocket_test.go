package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elodin/latency-dash/backend/calculator"
	"github.com/elodin/latency-dash/backend/proto"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// TestWebSocketServer tests the WebSocket server functionality
func TestWebSocketServer(t *testing.T) {
	calc := calculator.NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	wsServer := NewWebSocketServer(calc)

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(wsServer.HandleWebSocket))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect to the WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer conn.Close()

	// Test that client is registered
	wsServer.clientsMu.Lock()
	assert.Equal(t, 1, len(wsServer.clients), "Client should be registered")
	wsServer.clientsMu.Unlock()

	// Test broadcasting a metrics update
	update := &proto.MetricsUpdate{
		TargetId:    "test-target",
		Key:         "test-key",
		Min:         10.0,
		Max:         100.0,
		Avg:         50.0,
		P90:         90.0,
		Count:       10,
		LastUpdated: time.Now().UnixNano(),
		Metadata:    map[string]string{"tier": "test"},
	}

	wsServer.Broadcast(update)

	// Read the broadcast message
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var received proto.MetricsUpdate
	err = conn.ReadJSON(&received)
	assert.NoError(t, err, "Should receive broadcast message")
	assert.Equal(t, update.TargetId, received.TargetId)
	assert.Equal(t, update.Key, received.Key)
	assert.Equal(t, update.Min, received.Min)

	// Close connection
	conn.Close()

	// Test that client is deregistered
	time.Sleep(10 * time.Millisecond) // Allow cleanup to happen
	wsServer.clientsMu.Lock()
	assert.Equal(t, 0, len(wsServer.clients), "Client should be deregistered")
	wsServer.clientsMu.Unlock()
}

// TestWebSocketServerMultipleClients tests broadcasting to multiple clients
func TestWebSocketServerMultipleClients(t *testing.T) {
	calc := calculator.NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	wsServer := NewWebSocketServer(calc)

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(wsServer.HandleWebSocket))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect multiple clients
	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		conns[i] = conn
		defer conn.Close()
	}

	// Verify all clients are registered
	wsServer.clientsMu.Lock()
	assert.Equal(t, numClients, len(wsServer.clients), "All clients should be registered")
	wsServer.clientsMu.Unlock()

	// Test broadcasting to all clients
	update := &proto.MetricsUpdate{
		TargetId:    "test-target",
		Key:         "test-key",
		Min:         5.0,
		Max:         50.0,
		Avg:         25.0,
		P90:         45.0,
		Count:       20,
		LastUpdated: time.Now().UnixNano(),
		Metadata:    map[string]string{"region": "us-east"},
	}

	wsServer.Broadcast(update)

	// Verify all clients receive the message
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var received proto.MetricsUpdate
		err := conn.ReadJSON(&received)
		assert.NoError(t, err, "Client %d should receive broadcast message", i)
		assert.Equal(t, update.TargetId, received.TargetId)
		assert.Equal(t, update.Avg, received.Avg)
	}

	// Close one client
	conns[0].Close()
	time.Sleep(10 * time.Millisecond)

	// Verify client count is reduced
	wsServer.clientsMu.Lock()
	assert.Equal(t, numClients-1, len(wsServer.clients), "Client count should be reduced")
	wsServer.clientsMu.Unlock()

	// Broadcast again - remaining clients should still receive
	update2 := &proto.MetricsUpdate{
		TargetId:    "test-target",
		Key:         "test-key-2",
		Min:         1.0,
		Max:         10.0,
		Avg:         5.0,
		P90:         9.0,
		Count:       5,
		LastUpdated: time.Now().UnixNano(),
		Metadata:    map[string]string{"tier": "free"},
	}

	wsServer.Broadcast(update2)

	// Verify remaining clients receive the message
	for i := 1; i < numClients; i++ {
		conns[i].SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var received proto.MetricsUpdate
		err := conns[i].ReadJSON(&received)
		assert.NoError(t, err, "Client %d should receive second broadcast message", i)
		assert.Equal(t, update2.Key, received.Key)
		assert.Equal(t, update2.Avg, received.Avg)
	}
}

// TestWebSocketServerSubscriptionHandling tests subscription message processing
func TestWebSocketServerSubscriptionHandling(t *testing.T) {
	calc := calculator.NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	wsServer := NewWebSocketServer(calc)

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(wsServer.HandleWebSocket))
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect to the WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer conn.Close()

	// Send a subscription message
	subscription := proto.SubscriptionMessage{
		TargetId:         "test-target",
		SplitByMetadata: false,
		Keys:            []string{"key-1", "key-2"},
	}

	err = conn.WriteJSON(&subscription)
	assert.NoError(t, err, "Should be able to send subscription message")

	// The current implementation just logs the subscription, so we can't verify
	// the internal state easily. In a real implementation, we'd want to verify
	// that the subscription was processed correctly.

	// Close connection
	conn.Close()
}
