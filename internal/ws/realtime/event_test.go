package realtime

import (
	"testing"
	"time"
)

// TestEvent_DerivesChannelFromType asserts the channel is derived from the
// event type per the convention "<domain>.<event>" (e.g. "alerts.fired").
func TestEvent_DerivesChannelFromType(t *testing.T) {
	evt := NewEvent("alerts.fired", "alert-1", time.Now(), nil)
	if got, want := evt.Channel, "alerts.fired"; got != want {
		t.Fatalf("Channel = %q, want %q", got, want)
	}
}

// TestEvent_ChannelLowercased ensures mixed-case types are normalized to
// lowercase channel names.
func TestEvent_ChannelLowercased(t *testing.T) {
	evt := NewEvent("DeviceStateChanged", "device-1", time.Now(), nil)
	if got, want := evt.Channel, "devicestatechanged"; got != want {
		t.Fatalf("Channel = %q, want %q", got, want)
	}
}

// TestEvent_PreservesFields ensures struct fields round-trip.
func TestEvent_PreservesFields(t *testing.T) {
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	payload := map[string]any{"k": "v"}
	evt := NewEvent("metric.threshold_crossed", "metric-42", ts, payload)

	if evt.Type != "metric.threshold_crossed" {
		t.Fatalf("Type = %q", evt.Type)
	}
	if !evt.OccurredAt.Equal(ts) {
		t.Fatalf("OccurredAt = %v, want %v", evt.OccurredAt, ts)
	}
	if evt.SourceID != "metric-42" {
		t.Fatalf("SourceID = %q", evt.SourceID)
	}
	if evt.Payload["k"] != "v" {
		t.Fatalf("Payload[k] = %v", evt.Payload["k"])
	}
}

// TestEvent_PayloadImmutable makes sure payload map mutations after creation
// do not corrupt previously-published copies (publisher may pass the same map
// from multiple sources).
func TestEvent_PayloadCopied(t *testing.T) {
	original := map[string]any{"x": 1}
	evt := NewEvent("device.state_changed", "d-1", time.Now(), original)

	original["x"] = 2
	if evt.Payload["x"] != 1 {
		t.Fatalf("payload should be defensively copied, got %v", evt.Payload["x"])
	}
}
