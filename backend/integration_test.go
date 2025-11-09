package backend_test

import (
	"testing"
	"time"

	"github.com/elodin/latency-dash/backend/calculator"
	"github.com/elodin/latency-dash/backend/generator"
	"github.com/stretchr/testify/assert"
)

// TestEndToEndIntegration tests the complete pipeline from event generation to metric calculation
func TestEndToEndIntegration(t *testing.T) {
	// Initialize components
	calc := calculator.NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	// Subscribe to metrics updates
	sub := calc.Subscribe()
	defer calc.Unsubscribe(sub)

	// Create and start event generator
	config := generator.Config{
		TargetID:      "test-target",
		KeyPrefix:     "test-key-",
		NumKeys:       2,
		MinInterval:   10 * time.Millisecond,
		MaxInterval:   20 * time.Millisecond,
		MinPayload:    10,
		MaxPayload:    100,
		Metadata:      map[string]string{"tier": "test"},
		MetadataRules: map[string]map[string]float64{
			"tier": {"test": 1.0},
		},
	}

	gen := generator.NewEventGenerator(config)
	gen.Start()
	defer gen.Stop()

	// Forward generator events to calculator
	go func() {
		for event := range gen.Events() {
			calc.ProcessEvent(event)
		}
	}()

	// Wait for metrics update
	select {
	case update := <-sub:
		assert.Equal(t, "test-target", update.TargetId)
		assert.Contains(t, update.Key, "test-key-", "Key should contain test prefix")
		assert.GreaterOrEqual(t, update.Min, 0.0, "Min should be >= 0")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for metrics update")
	}
}

// TestMultipleGeneratorsIntegration tests integration with multiple generators
func TestMultipleGeneratorsIntegration(t *testing.T) {
	// Initialize components
	calc := calculator.NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	// Subscribe to metrics updates
	sub := calc.Subscribe()
	defer calc.Unsubscribe(sub)

	// Create multiple generators with different configurations
	configs := []generator.Config{
		{
			TargetID:      "target-1",
			KeyPrefix:     "service-a-",
			NumKeys:       2,
			MinInterval:   10 * time.Millisecond,
			MaxInterval:   20 * time.Millisecond,
			MinPayload:    20,
			MaxPayload:    200,
			Metadata:      map[string]string{"tier": "premium"},
			MetadataRules: map[string]map[string]float64{
				"tier": {"premium": 1.0},
			},
		},
		{
			TargetID:      "target-2",
			KeyPrefix:     "service-b-",
			NumKeys:       1,
			MinInterval:   15 * time.Millisecond,
			MaxInterval:   30 * time.Millisecond,
			MinPayload:    15,
			MaxPayload:    150,
			Metadata:      map[string]string{"tier": "free"},
			MetadataRules: map[string]map[string]float64{
				"tier": {"free": 1.5},
			},
		},
	}

	// Start all generators
	generators := make([]*generator.EventGenerator, len(configs))
	for i, config := range configs {
		gen := generator.NewEventGenerator(config)
		gen.Start()
		defer gen.Stop()
		generators[i] = gen

		// Forward events to calculator
		go func(g *generator.EventGenerator) {
			for event := range g.Events() {
				calc.ProcessEvent(event)
			}
		}(gen)
	}

	// Collect metrics updates from different targets
	targetUpdates := make(map[string]bool)
	timeout := time.After(10 * time.Second)

	for len(targetUpdates) < len(configs) {
		select {
		case update := <-sub:
			t.Logf("Received update for target %s (key: %s)", update.TargetId, update.Key)
			targetUpdates[update.TargetId] = true
		case <-timeout:
			t.Fatalf("Timeout waiting for metrics updates. Got updates for targets: %v", getMapKeys(targetUpdates))
		}
	}

	// Verify we received updates from all targets
	for _, config := range configs {
		_, exists := targetUpdates[config.TargetID]
		assert.True(t, exists, "Should receive updates for target %s", config.TargetID)
	}
}

// Helper function to get keys from a map
func getMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
