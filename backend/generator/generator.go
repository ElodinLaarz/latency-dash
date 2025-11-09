package generator

import (
	"math/rand"
	"sync"
	"time"

	"github.com/elodin/latency-dash/backend/proto"
)

type Config struct {
	TargetID      string
	KeyPrefix     string
	NumKeys       int
	MinInterval   time.Duration
	MaxInterval   time.Duration
	MinPayload    int
	MaxPayload    int
	Metadata      map[string]string
	MetadataRules map[string]map[string]float64 // Metadata-based latency multipliers
}

type EventGenerator struct {
	config    Config
	eventCh   chan *proto.Event
	stopCh    chan struct{}
	waitGroup sync.WaitGroup
}

func NewEventGenerator(config Config) *EventGenerator {
	return &EventGenerator{
		config:  config,
		eventCh: make(chan *proto.Event, 1000),
		stopCh:  make(chan struct{}),
	}
}

func (g *EventGenerator) Start() {
	g.waitGroup.Add(1)
	go g.run()
}

func (g *EventGenerator) Stop() {
	close(g.stopCh)
	g.waitGroup.Wait()
}

func (g *EventGenerator) Events() <-chan *proto.Event {
	return g.eventCh
}

func (g *EventGenerator) run() {
	defer g.waitGroup.Done()
	defer close(g.eventCh)

	for {
		select {
		case <-g.stopCh:
			return
		default:
			event := g.generateEvent()
			select {
			case g.eventCh <- event:
			case <-g.stopCh:
				return
			}

			// Calculate next interval with jitter
			interval := g.calculateInterval()
			time.Sleep(interval)
		}
	}
}

func (g *EventGenerator) generateEvent() *proto.Event {
	keyIndex := rand.Intn(g.config.NumKeys)
	key := g.config.KeyPrefix + string(rune('A'+keyIndex))
	
	// Calculate payload size with metadata-based adjustments
	payloadSize := g.calculatePayloadSize()
	payload := make([]byte, payloadSize)
	event := &proto.Event{
		TargetId:       g.config.TargetID,
		Key:            key,
		ServerTimestamp: time.Now().UnixNano(),
		Payload:        payload,
		PayloadSize:    int32(payloadSize),
		Metadata:       g.config.Metadata,
	}

	return event
}

func (g *EventGenerator) calculateInterval() time.Duration {
	baseInterval := g.config.MinInterval + time.Duration(rand.Float64()*float64(g.config.MaxInterval-g.config.MinInterval))
	
	// Apply metadata-based adjustments
	multiplier := 1.0
	for metaKey, metaValue := range g.config.Metadata {
		if rules, ok := g.config.MetadataRules[metaKey]; ok {
			if m, ok := rules[metaValue]; ok {
				multiplier *= m
			}
		}
	}

	// Ensure we don't go below minimum interval
	adjusted := time.Duration(float64(baseInterval) * multiplier)
	if adjusted < g.config.MinInterval {
		return g.config.MinInterval
	}
	return adjusted
}

func (g *EventGenerator) calculatePayloadSize() int {
	size := g.config.MinPayload + rand.Intn(g.config.MaxPayload-g.config.MinPayload)
	
	// Apply metadata-based adjustments
	for metaKey, metaValue := range g.config.Metadata {
		if rules, ok := g.config.MetadataRules[metaKey]; ok {
			if m, ok := rules[metaValue]; ok {
				size = int(float64(size) * m)
			}
		}
	}

	// Ensure we stay within bounds
	if size < g.config.MinPayload {
		return g.config.MinPayload
	}
	if size > g.config.MaxPayload {
		return g.config.MaxPayload
	}
	return size
}
