package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elodin/latency-dash/backend/calculator"
	"github.com/elodin/latency-dash/backend/generator"
	"github.com/elodin/latency-dash/backend/server"
)

func main() {
	// Initialize the metrics calculator
	metricsCalculator := calculator.NewMetricsCalculator()

	// Start the WebSocket server
	wsServer := server.NewWebSocketServer(metricsCalculator)

	// Set up HTTP routes
	http.HandleFunc("/ws", wsServer.HandleWebSocket)
	http.Handle("/", http.FileServer(http.Dir("../../frontend/dist")))

	// Start the HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		log.Printf("Server starting on :%s...\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start the metrics calculator
	metricsCalculator.Start()

	// Start test event generators
	startTestGenerators(metricsCalculator)

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
}

func startTestGenerators(calculator *calculator.MetricsCalculator) {
	// Define metadata rules for different tiers and regions
	metadataRules := map[string]map[string]float64{
		"tier": {
			"free":       1.5, // Free tier is 50% slower
			"premium":    1.0, // Baseline
			"enterprise": 0.7, // Enterprise is 30% faster
		},
		"region": {
			"us-east": 1.0,  // Baseline
			"us-west": 1.1,  // 10% slower
			"eu-west": 1.4,  // 40% slower
		},
	}

	// Create multiple test generators with different configurations
	configs := []generator.Config{
		{
			TargetID:    "prod-us-east",
			KeyPrefix:   "service-",
			NumKeys:     10,
			MinInterval: 100 * time.Millisecond,
			MaxInterval: 1000 * time.Millisecond,
			MinPayload:  100,
			MaxPayload:  1000,
			Metadata: map[string]string{
				"tier":   "enterprise",
				"region": "us-east",
			},
			MetadataRules: metadataRules,
		},
		{
			TargetID:    "prod-eu-west",
			KeyPrefix:   "service-",
			NumKeys:     8,
			MinInterval: 150 * time.Millisecond,
			MaxInterval: 1500 * time.Millisecond,
			MinPayload:  80,
			MaxPayload:  800,
			Metadata: map[string]string{
				"tier":   "premium",
				"region": "eu-west",
			},
			MetadataRules: metadataRules,
		},
	}

	// Start each generator
	for _, cfg := range configs {
		gen := generator.NewEventGenerator(cfg)
		gen.Start()

		// Forward events to the metrics calculator
		go func(g *generator.EventGenerator) {
			for event := range g.Events() {
				calculator.ProcessEvent(event)
			}
		}(gen)
	}
}
