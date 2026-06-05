package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Hub is the minimal surface the realtime package needs from the websocket
// hub. It mirrors the hub's Publish(channel, payload) method. Declaring it
// locally keeps this package framework-agnostic (no gorilla/websocket, no
// gin) and lets tests substitute a recording fake.
type Hub interface {
	Publish(channel string, payload []byte) error
}

// Publisher is the contract every domain service uses to broadcast events.
//
// Implementations MUST be safe for concurrent use; the websocket hub
// fan-out is concurrent and so are service-layer call sites.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// ErrNilHub is returned by HubPublisher.Publish when the wrapped hub is nil.
var ErrNilHub = errors.New("realtime: nil hub")

// HubPublisher forwards events to the websocket hub. The hub's own package
// owns connection and subscription state; this publisher is a thin adapter
// from Event to the hub's publish contract.
type HubPublisher struct {
	hub Hub
}

// NewHubPublisher wraps a Hub so it satisfies Publisher.
func NewHubPublisher(hub Hub) *HubPublisher {
	return &HubPublisher{hub: hub}
}

// Publish marshals the event to JSON and forwards it to the hub on the
// derived channel. The context is currently informational — the hub's
// Publish is synchronous — but is part of the interface so future async
// publishers can honor cancellation.
func (p *HubPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil || p.hub == nil {
		return ErrNilHub
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("realtime: marshal event: %w", err)
	}
	return p.hub.Publish(event.Channel, payload)
}

// NoopPublisher records published events in memory. It is intended for
// tests that need to assert on event shape and channel naming without
// standing up the websocket hub.
type NoopPublisher struct {
	mu     sync.Mutex
	events []Event
	err    error
}

// NewNoopPublisher returns a Publisher that records every event it
// receives. Optionally pass an error to be returned from every Publish
// (useful for testing error paths).
func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

// Publish records the event and returns the configured error (if any).
func (p *NoopPublisher) Publish(ctx context.Context, event Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}

// Events returns a snapshot of the events recorded so far.
func (p *NoopPublisher) Events() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Event, len(p.events))
	copy(out, p.events)
	return out
}
