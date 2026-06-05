// Package realtime bridges in-process domain events to the websocket hub.
//
// It is intentionally framework-agnostic: it does not import gin or
// gorilla/websocket. It depends only on a local Hub interface that matches
// the websocket hub's Publish method.
package realtime

import (
	"strings"
	"time"
)

// Event is the canonical structure published to the websocket hub.
//
// Channel is derived from Type using the "<domain>.<event>" convention
// (e.g. Type "alerts.fired" -> Channel "alerts.fired"). Lowercasing is
// applied so clients can subscribe with a stable identifier regardless of
// the caller's casing.
type Event struct {
	// Type is the event's logical identifier (e.g. "device.state_changed").
	Type string `json:"type"`
	// OccurredAt is the wall-clock time the event was produced.
	OccurredAt time.Time `json:"timestamp"`
	// SourceID is the originating entity's stable ID (device ID, alert ID, etc).
	SourceID string `json:"source_id"`
	// Payload carries event-specific data. A defensive copy is taken on
	// construction so subsequent caller mutations do not affect published
	// events.
	Payload map[string]any `json:"data"`
	// Channel is the hub channel the event is dispatched to.
	Channel string `json:"channel"`
}

// NewEvent builds an Event with Channel derived from Type.
func NewEvent(typ, sourceID string, occurredAt time.Time, payload map[string]any) Event {
	return Event{
		Type:      typ,
		OccurredAt: occurredAt,
		SourceID:  sourceID,
		Payload:   copyPayload(payload),
		Channel:   channelFor(typ),
	}
}

// channelFor maps a logical event type to its hub channel name.
//
// Convention: "<domain>.<event>" (already lower-case on the wire). When
// the caller supplies mixed case we normalize to lower case so subscribers
// can match deterministically.
func channelFor(typ string) string {
	return strings.ToLower(strings.TrimSpace(typ))
}

// copyPayload returns a shallow copy of the provided payload map so that
// later caller mutations do not leak into already-published events.
func copyPayload(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
