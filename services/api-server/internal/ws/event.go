package ws

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidEnvelope = errors.New("invalid event envelope")

type Event struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type OutboundEvent struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Timestamp string `json:"timestamp"`
	Payload   any    `json:"payload"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

func ParseEvent(data []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, ErrInvalidEnvelope
	}
	if event.Type == "" || event.Timestamp == "" || len(event.Payload) == 0 {
		return Event{}, ErrInvalidEnvelope
	}
	if _, err := time.Parse(time.RFC3339, event.Timestamp); err != nil {
		return Event{}, ErrInvalidEnvelope
	}

	payload := bytes.TrimSpace(event.Payload)
	if len(payload) < 2 || payload[0] != '{' || payload[len(payload)-1] != '}' {
		return Event{}, ErrInvalidEnvelope
	}

	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return Event{}, ErrInvalidEnvelope
	}

	return event, nil
}

func NewEvent(eventType string, requestID string, payload any) OutboundEvent {
	if payload == nil {
		payload = map[string]any{}
	}

	return OutboundEvent{
		Type:      eventType,
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
}

func NewErrorEvent(requestID string, code string, message string, details any) OutboundEvent {
	if details == nil {
		details = map[string]any{}
	}

	return NewEvent("error", requestID, ErrorPayload{
		Code:    code,
		Message: message,
		Details: details,
	})
}
