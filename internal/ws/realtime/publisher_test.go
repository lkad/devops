package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// recordingHub captures calls to Publish so tests can assert behavior.
type recordingHub struct {
	mu       sync.Mutex
	calls    []hubCall
	failNext error
}

type hubCall struct {
	channel string
	payload []byte
}

func (h *recordingHub) Publish(channel string, payload []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failNext != nil {
		err := h.failNext
		h.failNext = nil
		return err
	}
	h.calls = append(h.calls, hubCall{channel: channel, payload: append([]byte(nil), payload...)})
	return nil
}

func (h *recordingHub) Calls() []hubCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]hubCall, len(h.calls))
	copy(out, h.calls)
	return out
}

// TestPublisher_InterfaceCompilation ensures NoopPublisher and HubPublisher
// satisfy the Publisher interface.
func TestPublisher_InterfaceCompilation(t *testing.T) {
	var _ Publisher = (*NoopPublisher)(nil)
	var _ Publisher = (*HubPublisher)(nil)
}

// TestNoopPublisher_RecordsEvents asserts NoopPublisher captures events
// without forwarding them to any hub.
func TestNoopPublisher_RecordsEvents(t *testing.T) {
	p := NewNoopPublisher()
	ctx := context.Background()

	evt := NewEvent("alerts.fired", "a-1", time.Now(), map[string]any{"k": "v"})
	if err := p.Publish(ctx, evt); err != nil {
		t.Fatalf("Publish err = %v", err)
	}

	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "alerts.fired" {
		t.Fatalf("event type = %q", events[0].Type)
	}
}

// TestHubPublisher_PublishesToHub ensures HubPublisher translates Event into
// a hub publish call carrying the channel name and JSON payload.
func TestHubPublisher_PublishesToHub(t *testing.T) {
	hub := &recordingHub{}
	p := NewHubPublisher(hub)

	ts := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	evt := NewEvent("device.state_changed", "dev-1", ts, map[string]any{"state": "online"})
	if err := p.Publish(context.Background(), evt); err != nil {
		t.Fatalf("Publish err = %v", err)
	}

	calls := hub.Calls()
	if len(calls) != 1 {
		t.Fatalf("hub calls = %d, want 1", len(calls))
	}
	if calls[0].channel != "device.state_changed" {
		t.Fatalf("channel = %q, want %q", calls[0].channel, "device.state_changed")
	}
	if len(calls[0].payload) == 0 {
		t.Fatalf("payload is empty")
	}
}

// TestHubPublisher_PropagatesError ensures errors from the hub surface
// to the caller.
func TestHubPublisher_PropagatesError(t *testing.T) {
	hub := &recordingHub{failNext: errors.New("hub down")}
	p := NewHubPublisher(hub)

	evt := NewEvent("alerts.resolved", "a-1", time.Now(), nil)
	if err := p.Publish(context.Background(), evt); err == nil {
		t.Fatalf("expected error from hub")
	}
}

// TestHubPublisher_NilHub ensures a nil hub yields a clear error rather
// than a nil-pointer panic.
func TestHubPublisher_NilHub(t *testing.T) {
	p := NewHubPublisher(nil)
	err := p.Publish(context.Background(), NewEvent("x", "1", time.Now(), nil))
	if err == nil {
		t.Fatalf("expected error for nil hub")
	}
}
