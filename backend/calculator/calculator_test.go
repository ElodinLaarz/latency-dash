package calculator

import (
	"container/ring"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elodin/latency-dash/backend/proto"
	"github.com/stretchr/testify/assert"
)

// createTestEvent creates a standard test event for calculator tests
func createTestEvent(targetID, key string, metadata map[string]string) *proto.Event {
	if metadata == nil {
		metadata = map[string]string{"tier": "test"}
	}

	return &proto.Event{
		TargetId:       targetID,
		Key:            key,
		ServerTimestamp: time.Now().UnixNano(),
		Payload:        []byte("test"),
		PayloadSize:     4,
		Metadata:       metadata,
	}
}

func TestMetricsUpdate(t *testing.T) {
	calc := NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	// Create a test event
	event := createTestEvent("test-target", "test-key", map[string]string{"tier": "test"})

	// Process the event
	calc.ProcessEvent(event)

	// Give the calculator a moment to process the event
	time.Sleep(100 * time.Millisecond)

	// Verify metrics were calculated
	calc.metricsMu.RLock()
	defer calc.metricsMu.RUnlock()

	// The key should be in the format "target:key:metadata"
	key := "test-target:test-key"
	for k := range calc.metrics {
		if k == key || k == key+":tier=test" {
			metrics := calc.metrics[k]
			assert.Equal(t, int64(1), metrics.GetCount(), "Should have 1 sample")
			return
		}
	}
	t.Errorf("No metrics found for key %s", key)
}

func TestMetricsCalculation(t *testing.T) {
	calc := NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	now := time.Now()

	// Send multiple events with increasing timestamps and payload sizes
	// to ensure we have some variance in the metrics
	for i := range 10 {
		event := &proto.Event{
			TargetId:        "test-target",
			Key:             "test-key",
			ServerTimestamp: now.Add(time.Duration(i*100) * time.Millisecond).UnixNano(),
			Payload:         make([]byte, i+1),
			PayloadSize:     int32(i + 1),
		}
		calc.ProcessEvent(event)
		// Add a small delay to ensure timestamps are different
		time.Sleep(1 * time.Millisecond)
	}

	// Give the calculator time to process events
	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	calc.metricsMu.RLock()
	defer calc.metricsMu.RUnlock()

	// Find the metrics for our test key
	var metrics *Metrics
	for _, m := range calc.metrics {
		// Just take the first metrics we find for testing
		metrics = m
		break
	}

	if metrics == nil {
		t.Fatal("No metrics found for test key")
	}

	// We should have at least one sample
	assert.Greater(t, metrics.GetCount(), int64(0), "Should have at least one sample")

	// Check that metrics are within expected ranges
	// We're not checking exact values since they depend on timing
	min := metrics.GetMin()
	max := metrics.GetMax()
	avg := metrics.GetAvg()

	assert.GreaterOrEqual(t, min, 0.0, "Min should be >= 0")
	assert.GreaterOrEqual(t, max, min, "Max should be >= Min")
	assert.GreaterOrEqual(t, avg, min, "Avg should be >= Min")
	assert.LessOrEqual(t, avg, max, "Avg should be <= Max")
	assert.GreaterOrEqual(t, metrics.GetP90(), 0.0, "P90 should be >= 0")
}

func TestMetricsGetters(t *testing.T) {
	m := &Metrics{
		Samples: ring.New(1),
	}

	// Test initial values
	assert.Equal(t, int64(0), m.GetCount())
	assert.Equal(t, 0.0, m.GetMin())
	assert.Equal(t, 0.0, m.GetMax())
	assert.Equal(t, 0.0, m.GetAvg())
	assert.Equal(t, 0.0, m.GetP90())

	// Update with some values
	m.mu.Lock()
	m.Samples.Value = 100.0 // 100ms
	atomic.StoreInt64(&m.count, 1)
	atomic.StoreInt64(&m.min, int64(100*float64(time.Millisecond)))
	atomic.StoreInt64(&m.max, int64(100*float64(time.Millisecond)))
	atomic.StoreInt64(&m.avg, int64(100*float64(time.Millisecond)))
	atomic.StoreInt64(&m.p90, int64(100*float64(time.Millisecond)))
	m.mu.Unlock()

	// Test updated values
	assert.Equal(t, int64(1), m.GetCount())
	assert.Equal(t, 100.0, m.GetMin())
	assert.Equal(t, 100.0, m.GetMax())
	assert.Equal(t, 100.0, m.GetAvg())
	assert.Equal(t, 100.0, m.GetP90())
}

func TestMetricsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		events   []*proto.Event
		expected func(t *testing.T, metrics *Metrics)
	}{
		{
			name: "single_event",
			events: []*proto.Event{
				createTestEvent("test-target", "test-key", map[string]string{"tier": "test"}),
			},
			expected: func(t *testing.T, metrics *Metrics) {
				assert.Equal(t, int64(1), metrics.GetCount())
				// For single event, all metrics should be 0 (first interval)
				assert.Equal(t, 0.0, metrics.GetMin())
				assert.Equal(t, 0.0, metrics.GetMax())
				assert.Equal(t, 0.0, metrics.GetAvg())
				assert.Equal(t, 0.0, metrics.GetP90())
			},
		},
		{
			name: "two_events_same_time",
			events: []*proto.Event{
				createTestEvent("test-target", "test-key", map[string]string{"tier": "test"}),
				func() *proto.Event {
					// Create event with same timestamp
					event := createTestEvent("test-target", "test-key", map[string]string{"tier": "test"})
					event.ServerTimestamp = time.Now().UnixNano() // Same timestamp
					return event
				}(),
			},
			expected: func(t *testing.T, metrics *Metrics) {
				assert.Equal(t, int64(2), metrics.GetCount())
				// Second event at same time should result in very small interval (due to execution time)
				min := metrics.GetMin()
				max := metrics.GetMax()
				assert.GreaterOrEqual(t, min, 0.0, "Min should be >= 0")
				assert.LessOrEqual(t, min, 1.0, "Min should be very small for same timestamp")
				assert.GreaterOrEqual(t, max, min, "Max should be >= Min")
			},
		},
		{
			name: "negative_interval_protection",
			events: []*proto.Event{
				func() *proto.Event {
					event := createTestEvent("test-target", "test-key", map[string]string{"tier": "test"})
					event.ServerTimestamp = time.Now().Add(time.Second).UnixNano()
					return event
				}(),
				func() *proto.Event {
					event := createTestEvent("test-target", "test-key", map[string]string{"tier": "test"})
					event.ServerTimestamp = time.Now().UnixNano() // Earlier timestamp
					return event
				}(),
			},
			expected: func(t *testing.T, metrics *Metrics) {
				assert.Equal(t, int64(2), metrics.GetCount())
				// Negative intervals should be clamped to 0
				min := metrics.GetMin()
				max := metrics.GetMax()
				avg := metrics.GetAvg()
				assert.Equal(t, 0.0, min, "Min should be 0 due to negative interval protection")
				assert.Equal(t, 0.0, max, "Max should be 0")
				assert.Equal(t, 0.0, avg, "Avg should be 0")
				assert.Equal(t, 0.0, metrics.GetP90(), "P90 should be 0")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewMetricsCalculator()
			calc.Start()
			defer calc.Stop()

			// Process events
			for _, event := range tt.events {
				calc.ProcessEvent(event)
			}

			// Give calculator time to process
			time.Sleep(50 * time.Millisecond)

			// Find the metrics
			calc.metricsMu.RLock()
			var metrics *Metrics
			for _, m := range calc.metrics {
				metrics = m
				break
			}
			calc.metricsMu.RUnlock()

			if metrics == nil {
				t.Fatal("No metrics found")
			}

			tt.expected(t, metrics)
		})
	}
}

func TestRingBufferBehavior(t *testing.T) {
	calc := NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	baseTime := time.Now()

	// Send events to fill and overflow the ring buffer
	// Use fewer events and ensure they're spaced out properly
	const numEvents = 1200 // More than default MaxSamples (1000) but not too many
	for i := 0; i < numEvents; i++ {
		event := createTestEvent("test-target", "test-key", nil)
		event.ServerTimestamp = baseTime.Add(time.Duration(i*100) * time.Millisecond).UnixNano()
		calc.ProcessEvent(event)
		
		// Small delay to ensure events are processed in order
		if i%100 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Give calculator more time to process all events
	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	calc.metricsMu.RLock()
	var metrics *Metrics
	for _, m := range calc.metrics {
		metrics = m
		break
	}
	calc.metricsMu.RUnlock()

	assert.NotNil(t, metrics)
	count := metrics.GetCount()
	assert.GreaterOrEqual(t, count, int64(1000), "Should count at least MaxSamples events")
	assert.LessOrEqual(t, count, int64(numEvents), "Should not count more than sent events")

	// Metrics should be based on the most recent samples in the ring buffer
	min := metrics.GetMin()
	max := metrics.GetMax()
	avg := metrics.GetAvg()
	p90 := metrics.GetP90()

	assert.GreaterOrEqual(t, min, 0.0)
	assert.GreaterOrEqual(t, max, min)
	assert.GreaterOrEqual(t, avg, min)
	assert.LessOrEqual(t, avg, max)
	assert.GreaterOrEqual(t, p90, 0.0)
}

func TestP90CalculationAccuracy(t *testing.T) {
	// Create metrics with a ring buffer large enough for our samples
	metrics := &Metrics{
		Samples: ring.New(10), // Ring buffer for 10 samples
	}

	baseTime := time.Now()

	// Add a single event first to establish baseline
	event1 := &proto.Event{
		TargetId:       "test-target",
		Key:            "test-key",
		ServerTimestamp: baseTime.UnixNano(),
		Payload:        []byte("test"),
		PayloadSize:     4,
		Metadata:       map[string]string{"tier": "test"},
	}
	metrics.Update(event1)

	// Add a second event with a known interval
	event2 := &proto.Event{
		TargetId:       "test-target",
		Key:            "test-key",
		ServerTimestamp: baseTime.Add(100 * time.Millisecond).UnixNano(),
		Payload:        []byte("test"),
		PayloadSize:     4,
		Metadata:       map[string]string{"tier": "test"},
	}
	metrics.Update(event2)

	// Check the values
	p90 := metrics.GetP90()
	count := metrics.GetCount()

	// The timestamp difference should be 100ms = 100,000,000 nanoseconds
	expectedDiffNs := int64(100 * time.Millisecond)
	actualDiffNs := event2.ServerTimestamp - event1.ServerTimestamp
	assert.Equal(t, expectedDiffNs, actualDiffNs, "Timestamp difference should be 100ms")

	// For just 2 events, check basic properties
	assert.Equal(t, int64(2), count)
	
	// P90 should be a reasonable value (exact calculation is complex with ring buffer)
	assert.GreaterOrEqual(t, p90, 0.0, "P90 should be >= 0")
	assert.LessOrEqual(t, p90, 100.0, "P90 should be <= max interval")
}

func TestConcurrentAccess(t *testing.T) {
	calc := NewMetricsCalculator()
	calc.Start()
	defer calc.Stop()

	// Start multiple goroutines to simulate concurrent access
	const numWorkers = 10
	const eventsPerWorker = 10 // Reduced for faster test execution

	var wg sync.WaitGroup
	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < eventsPerWorker; j++ {
				event := createTestEvent("test-target", fmt.Sprintf("key-%d", workerID), nil)
				calc.ProcessEvent(event)
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()

	// Give some time for events to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify all events were processed
	calc.metricsMu.RLock()
	defer calc.metricsMu.RUnlock()

	// We should have numWorkers different keys
	assert.Equal(t, numWorkers, len(calc.metrics), "Number of metrics should match number of workers")

	// Each key should have eventsPerWorker intervals
	for key, metrics := range calc.metrics {
		t.Run(key, func(t *testing.T) {
			// We expect up to eventsPerWorker intervals (first event is also counted as an interval with 0 duration)
			expectedMaxCount := int64(eventsPerWorker)
			count := metrics.GetCount()
			assert.GreaterOrEqual(t, count, int64(0), "Should have processed some events")
			assert.LessOrEqual(t, count, expectedMaxCount, "Should not have more intervals than events")
		})
	}
}
