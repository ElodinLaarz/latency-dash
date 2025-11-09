package backend

import (
	"time"

	"github.com/elodin/latency-dash/backend/proto"
)

// Test helper functions and fixtures for backend tests

// CreateTestEvent creates a standard test event with customizable parameters
func CreateTestEvent(targetID, key string, timestamp time.Time, metadata map[string]string) *proto.Event {
	if metadata == nil {
		metadata = map[string]string{"tier": "test"}
	}

	return &proto.Event{
		TargetId:       targetID,
		Key:            key,
		ServerTimestamp: timestamp.UnixNano(),
		Payload:        []byte("test payload"),
		PayloadSize:     int32(len("test payload")),
		Metadata:       metadata,
	}
}

// CreateTestEventWithInterval creates a test event with a specific time interval from base time
func CreateTestEventWithInterval(targetID, key string, baseTime time.Time, intervalMs int) *proto.Event {
	timestamp := baseTime.Add(time.Duration(intervalMs) * time.Millisecond)
	return CreateTestEvent(targetID, key, timestamp, nil)
}

// CreateSubscriptionMessage creates a test subscription message
func CreateSubscriptionMessage(targetIds, keys []string) *proto.SubscriptionMessage {
	return &proto.SubscriptionMessage{
		TargetId:        targetIds[0], // Use first target ID for now
		SplitByMetadata: false,
		Keys:           keys,
	}
}
