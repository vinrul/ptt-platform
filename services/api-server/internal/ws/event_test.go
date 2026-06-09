package ws

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseEventAcceptsValidEnvelope(t *testing.T) {
	event, err := ParseEvent([]byte(`{
		"type":"heartbeat",
		"requestId":"req-1",
		"timestamp":"2026-06-09T12:00:00Z",
		"payload":{}
	}`))
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if event.Type != "heartbeat" {
		t.Fatalf("expected heartbeat type, got %s", event.Type)
	}
}

func TestParseEventRejectsInvalidEnvelope(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"timestamp":"2026-06-09T12:00:00Z","payload":{}}`),
		[]byte(`{"type":"heartbeat","timestamp":"invalid","payload":{}}`),
		[]byte(`{"type":"heartbeat","timestamp":"2026-06-09T12:00:00Z","payload":[]}`),
	}

	for _, data := range cases {
		if _, err := ParseEvent(data); err == nil {
			t.Fatalf("expected invalid envelope for %s", data)
		}
	}
}

func TestNewEventUsesUTCServerTimestamp(t *testing.T) {
	event := NewEvent("connection.ready", "", map[string]any{})
	parsed, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		t.Fatalf("parse timestamp: %v", err)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("expected UTC timestamp, got %s", parsed.Location())
	}
	if _, err := json.Marshal(event); err != nil {
		t.Fatalf("marshal event: %v", err)
	}
}
