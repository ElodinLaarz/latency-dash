package calculator

import (
	"container/ring"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elodin/latency-dash/backend/proto"
)

const (
	// MaxSamples is the maximum number of samples to keep for each key
	MaxSamples = 1000
)

type Metrics struct {
	TargetID string
	Key      string
	Metadata map[string]string

	Samples *ring.Ring // Circular buffer of recent samples
	mu      sync.RWMutex
	
	// All fields below are accessed atomically
	count int64 // Number of samples
	min   int64 // Minimum latency in milliseconds (stored as int64 to use atomic operations)
	max   int64 // Maximum latency in milliseconds (stored as int64 to use atomic operations)
	avg   int64 // Average latency in milliseconds (stored as int64 to use atomic operations)
	p90   int64 // 90th percentile latency in milliseconds (stored as int64 to use atomic operations)
}

// GetCount returns the current count of samples (thread-safe)
func (m *Metrics) GetCount() int64 {
	return atomic.LoadInt64(&m.count)
}

// GetMin returns the minimum latency in milliseconds (thread-safe)
func (m *Metrics) GetMin() float64 {
	return float64(atomic.LoadInt64(&m.min)) / float64(time.Millisecond)
}

// GetMax returns the maximum latency in milliseconds (thread-safe)
func (m *Metrics) GetMax() float64 {
	return float64(atomic.LoadInt64(&m.max)) / float64(time.Millisecond)
}

// GetAvg returns the average latency in milliseconds (thread-safe)
func (m *Metrics) GetAvg() float64 {
	return float64(atomic.LoadInt64(&m.avg)) / float64(time.Millisecond)
}

// GetP90 returns the 90th percentile latency in milliseconds (thread-safe)
func (m *Metrics) GetP90() float64 {
	return float64(atomic.LoadInt64(&m.p90)) / float64(time.Millisecond)
}

type MetricsCalculator struct {
	metrics   map[string]*Metrics // key: targetID:key:metadataHash
	metricsMu sync.RWMutex

	updateCh      chan *proto.Event
	subscribers   map[chan *proto.MetricsUpdate]struct{}
	subscribersMu sync.RWMutex

	stopOnce sync.Once
	stopped  bool
	stopMu   sync.RWMutex
}

func NewMetricsCalculator() *MetricsCalculator {
	return &MetricsCalculator{
		metrics:     make(map[string]*Metrics),
		updateCh:    make(chan *proto.Event, 1000),
		subscribers: make(map[chan *proto.MetricsUpdate]struct{}),
	}
}

func (c *MetricsCalculator) Start() {
	go c.processEvents()
}

func (c *MetricsCalculator) ProcessEvent(event *proto.Event) {
	c.stopMu.RLock()
	defer c.stopMu.RUnlock()
	if c.stopped {
		return
	}
	c.updateCh <- event
}

func (c *MetricsCalculator) Subscribe() chan *proto.MetricsUpdate {
	ch := make(chan *proto.MetricsUpdate, 100)
	c.subscribersMu.Lock()
	c.subscribers[ch] = struct{}{}
	c.subscribersMu.Unlock()
	return ch
}

func (c *MetricsCalculator) Unsubscribe(ch chan *proto.MetricsUpdate) {
	c.subscribersMu.Lock()
	delete(c.subscribers, ch)
	close(ch)
	c.subscribersMu.Unlock()
}

func (c *MetricsCalculator) Stop() {
	c.stopOnce.Do(func() {
		c.stopMu.Lock()
		c.stopped = true
		c.stopMu.Unlock()
		close(c.updateCh)
	})
}

func (c *MetricsCalculator) processEvents() {
	for event := range c.updateCh {
		metrics := c.getOrCreateMetrics(event)
		metrics.Update(event)

		// Create and send update to subscribers
		update := &proto.MetricsUpdate{
			TargetId:    event.TargetId,
			Key:         event.Key,
			Min:         metrics.GetMin(),
			Max:         metrics.GetMax(),
			Avg:         metrics.GetAvg(),
			P90:         metrics.GetP90(),
			Count:       metrics.GetCount(),
			LastUpdated: time.Now().UnixNano(),
			Metadata:    event.Metadata,
		}

		c.notifySubscribers(update)
	}
}

func (c *MetricsCalculator) getOrCreateMetrics(event *proto.Event) *Metrics {
	// Create a unique key for this target + key + metadata combination
	key := event.TargetId + ":" + event.Key
	if len(event.Metadata) > 0 {
		// Simple hash of metadata for key uniqueness
		for k, v := range event.Metadata {
			key += ":" + k + "=" + v
		}
	}

	c.metricsMu.RLock()
	metrics, exists := c.metrics[key]
	c.metricsMu.RUnlock()

	if !exists {
		metrics = &Metrics{
			TargetID: event.TargetId,
			Key:      event.Key,
			Metadata: event.Metadata,
			Samples:  ring.New(MaxSamples),
		}

		c.metricsMu.Lock()
		c.metrics[key] = metrics
		c.metricsMu.Unlock()
	}

	return metrics
}

func (c *MetricsCalculator) notifySubscribers(update *proto.MetricsUpdate) {
	c.subscribersMu.RLock()
	defer c.subscribersMu.RUnlock()

	for ch := range c.subscribers {
		select {
		case ch <- update:
		default:
			// Drop message if subscriber's channel is full to prevent blocking
		}
	}
}

func (m *Metrics) Update(event *proto.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Calculate time since last event for this key
	var intervalMs float64
	count := atomic.LoadInt64(&m.count)
	currentTimeMs := float64(event.ServerTimestamp) / float64(time.Millisecond)

	if count > 0 {
		// For subsequent events, calculate the interval since the last event
		lastTime := m.Samples.Value
		if lastTime != nil {
			// Get the last timestamp from the ring buffer (stored in milliseconds)
			lastEventTimeMs := lastTime.(float64)
			intervalMs = currentTimeMs - lastEventTimeMs
			// Ensure interval is non-negative
			if intervalMs < 0 {
				intervalMs = 0
			}
		}
	} else {
		// For the first event, use a default interval of 0
		intervalMs = 0
	}

	// Store the current timestamp in milliseconds in the circular buffer
	m.Samples = m.Samples.Next()
	m.Samples.Value = currentTimeMs

	// Convert interval to nanoseconds for atomic operations (storing as int64)
	intervalNs := int64(intervalMs * float64(time.Millisecond))

	// Update stats atomically
	if count == 0 {
		atomic.StoreInt64(&m.min, intervalNs)
		atomic.StoreInt64(&m.max, intervalNs)
		atomic.StoreInt64(&m.avg, intervalNs)
	} else {
		// Update min
		for {
			currentMin := atomic.LoadInt64(&m.min)
			if intervalNs >= currentMin {
				break
			}
			if atomic.CompareAndSwapInt64(&m.min, currentMin, intervalNs) {
				break
			}
		}

		// Update max
		for {
			currentMax := atomic.LoadInt64(&m.max)
			if intervalNs <= currentMax {
				break
			}
			if atomic.CompareAndSwapInt64(&m.max, currentMax, intervalNs) {
				break
			}
		}

		// Update average
		for {
			currentAvg := atomic.LoadInt64(&m.avg)
			newAvg := (currentAvg*count + intervalNs) / (count + 1)
			if atomic.CompareAndSwapInt64(&m.avg, currentAvg, newAvg) {
				break
			}
		}
	}

	// Increment count atomically
	atomic.AddInt64(&m.count, 1)

	// Calculate P90
	p90 := m.calculatePercentile(90)
	atomic.StoreInt64(&m.p90, int64(p90 * float64(time.Millisecond)))
}

func (m *Metrics) calculatePercentile(p float64) float64 {
	count := atomic.LoadInt64(&m.count)
	if count <= 1 {
		return float64(atomic.LoadInt64(&m.avg)) / float64(time.Millisecond)
	}

	// Collect intervals from consecutive timestamps in the ring buffer
	samples := make([]float64, 0, count-1) // We have count-1 intervals for count events
	r := m.Samples

	// Start from the oldest timestamp and work backwards to calculate intervals
	timestamps := make([]float64, 0, count)
	for i := 0; i < int(count); i++ {
		if r.Value != nil {
			timestamps = append(timestamps, r.Value.(float64))
		}
		r = r.Next()
	}

	// Calculate intervals between consecutive timestamps
	for i := 1; i < len(timestamps); i++ {
		interval := timestamps[i] - timestamps[i-1]
		if interval >= 0 { // Only include non-negative intervals
			samples = append(samples, interval)
		}
	}

	if len(samples) == 0 {
		return 0
	}

	// Sort samples
	sort.Float64s(samples)

	// Calculate index for the percentile
	index := int(float64(len(samples)-1) * p / 100.0)
	if index >= len(samples) {
		index = len(samples) - 1
	}
	return samples[index]
}
