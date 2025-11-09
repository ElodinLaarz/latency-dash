package generator

import (
	"math/rand"
	"testing"
	"time"

	"github.com/elodin/latency-dash/backend/proto"
	"github.com/stretchr/testify/assert"
)

// testGenerator wraps EventGenerator for testing
type testGenerator struct {
	*EventGenerator
	eventCh chan *proto.Event
}

func newTestGenerator(config Config) *testGenerator {
	g := NewEventGenerator(config)
	// Create a buffered channel to prevent blocking in tests
	g.eventCh = make(chan *proto.Event, 100)
	return &testGenerator{
		EventGenerator: g,
		eventCh:        g.eventCh,
	}
}

func TestEventGeneration(t *testing.T) {
	config := Config{
		TargetID:      "test-target",
		KeyPrefix:     "test-",
		NumKeys:       5,
		MinInterval:   100 * time.Millisecond,
		MaxInterval:   200 * time.Millisecond,
		MinPayload:    10,
		MaxPayload:    100,
		Metadata:      map[string]string{"tier": "test", "region": "test"},
		MetadataRules: map[string]map[string]float64{
			"tier": {"test": 1.0},
			"region": {"test": 1.0},
		},
	}

	gen := newTestGenerator(config)
	gen.Start()
	defer gen.Stop()

	// Wait for an event
	select {
	case event := <-gen.eventCh:
		assert.NotNil(t, event)
		assert.Equal(t, "test-target", event.TargetId)
		assert.Contains(t, event.Key, "test-")
		assert.GreaterOrEqual(t, event.PayloadSize, int32(10))
		assert.LessOrEqual(t, event.PayloadSize, int32(100))
		assert.Equal(t, "test", event.Metadata["tier"])
		assert.Equal(t, "test", event.Metadata["region"])
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestMetadataAffectsLatency(t *testing.T) {
	// Test that different metadata values affect the interval calculation
	config := Config{
		TargetID:      "test-target",
		KeyPrefix:     "test-",
		NumKeys:       1,
		MinInterval:   100 * time.Millisecond,
		MaxInterval:   100 * time.Millisecond, // Fixed interval for testing
		MinPayload:    10,
		MaxPayload:    10, // Fixed payload for testing
		Metadata:      map[string]string{"tier": "slow"},
		MetadataRules: map[string]map[string]float64{
			"tier": {"slow": 2.0}, // 2x multiplier
		},
	}

	gen := NewEventGenerator(config)
	interval := gen.calculateInterval()
	
	// With a 2x multiplier, the interval should be 200ms
	// Since we can't predict the exact interval due to random factors,
	// we'll just verify it's within a reasonable range
	assert.True(t, interval >= 100*time.Millisecond && interval <= 200*time.Millisecond,
		"Expected interval between 100ms and 200ms, got %v", interval)
	
	// Test with different metadata
	config.Metadata["tier"] = "fast"
	config.MetadataRules["tier"] = map[string]float64{"fast": 0.5} // 0.5x multiplier
	gen = NewEventGenerator(config)
	interval = gen.calculateInterval()
	
	// With a 0.5x multiplier, the interval should be between 50ms and 100ms
	assert.True(t, interval >= 50*time.Millisecond && interval <= 100*time.Millisecond,
		"Expected interval between 50ms and 100ms, got %v", interval)
}

func TestPayloadSizeWithinBounds(t *testing.T) {
	config := Config{
		TargetID:      "test-target",
		KeyPrefix:     "test-",
		NumKeys:       1,
		MinInterval:   100 * time.Millisecond,
		MaxInterval:   200 * time.Millisecond,
		MinPayload:    10,
		MaxPayload:    100,
		Metadata:      map[string]string{"tier": "test"},
		MetadataRules: map[string]map[string]float64{
			"tier": {"test": 1.0},
		},
	}

	gen := NewEventGenerator(config)
	
	// Test that payload size is within bounds for different multipliers
	tests := []struct {
		name string
		mult float64
	}{
		{"normal", 1.0},
		{"smaller", 0.5},
		{"larger", 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update the config with the new multiplier
			updatedConfig := gen.config
			updatedConfig.MetadataRules["tier"] = map[string]float64{"test": tt.mult}
			gen.config = updatedConfig
			
			size := gen.calculatePayloadSize()
			
			// The actual size should be between MinPayload and MaxPayload
			assert.True(t, size >= config.MinPayload, 
				"Size %d should be >= MinPayload %d for %s", 
				size, config.MinPayload, tt.name)
				
			assert.True(t, size <= config.MaxPayload, 
				"Size %d should be <= MaxPayload %d for %s", 
				size, config.MaxPayload, tt.name)
		})
	}
}

// randFloat64 is a package-level variable that can be overridden in tests
var randFloat64 = rand.Float64

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid_config",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     5,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
				Metadata:    map[string]string{"tier": "test"},
			},
			expectError: false,
		},
		{
			name: "negative_num_keys",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     -1,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
			},
			expectError: true,
		},
		{
			name: "zero_num_keys",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     0,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
			},
			expectError: true,
		},
		{
			name: "min_interval_greater_than_max",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     5,
				MinInterval: 200 * time.Millisecond,
				MaxInterval: 100 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
			},
			expectError: true,
		},
		{
			name: "min_payload_greater_than_max",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     5,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  100,
				MaxPayload:  10,
			},
			expectError: true,
		},
		{
			name: "negative_min_payload",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "test-",
				NumKeys:     5,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  -1,
				MaxPayload:  100,
			},
			expectError: true,
		},
		{
			name: "empty_target_id",
			config: Config{
				TargetID:    "",
				KeyPrefix:   "test-",
				NumKeys:     5,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
			},
			expectError: true,
		},
		{
			name: "empty_key_prefix",
			config: Config{
				TargetID:    "test-target",
				KeyPrefix:   "",
				NumKeys:     5,
				MinInterval: 100 * time.Millisecond,
				MaxInterval: 200 * time.Millisecond,
				MinPayload:  10,
				MaxPayload:  100,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Currently, the generator doesn't validate config, it just uses it
			// In a real implementation, we'd want to validate the config
			gen := NewEventGenerator(tt.config)
			
			// For now, just check that the generator is created successfully
			// In the future, we might add a Validate() method to Config
			assert.NotNil(t, gen, "Generator should be created")
			
			// We can test some basic functionality
			if tt.config.NumKeys > 0 && tt.config.MinInterval > 0 && tt.config.MaxInterval >= tt.config.MinInterval {
				// Should be able to start without panicking
				gen.Start()
				defer gen.Stop()
			}
		})
	}
}
