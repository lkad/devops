// Package hub implements the central WebSocket connection broker: the
// single goroutine-safe registry of clients, their channel subscriptions,
// and the publish path that dispatches messages to subscribers.
//
// Limits captures the resource caps enforced by the hub. The defaults
// come from the spec (30s/10s/60s keepalive; 256-deep Send buffer; etc.)
// and the Validate method is the single source of truth for what
// counts as a sensible value.
package hub

import (
	"errors"
	"fmt"
)

// DefaultOutBufferSize is the per-client Send channel buffer depth.
// The spec pins this at 256 so a slow consumer can absorb a small
// burst before the hub disconnects it for queue overflow.
const DefaultOutBufferSize = 256

// Limits groups every resource cap the hub enforces. Timeouts are in
// seconds so the type can travel through the public Config loader
// (config.WebSocketConfig) without depending on time.Duration.
type Limits struct {
	// MaxConnections is the hard cap on simultaneously-connected clients.
	// The second connection attempt is rejected when this is reached.
	MaxConnections int
	// MaxChannelsPerConn is the upper bound on distinct channels a single
	// client may subscribe to at one time.
	MaxChannelsPerConn int
	// MaxMessageBytes is the largest frame the read pump will accept
	// from a client; oversized frames are dropped and the client is
	// disconnected.
	MaxMessageBytes int64
	// MaxPayloadBytes is the largest payload a single Publish call may
	// carry; oversize payloads are dropped at the hub boundary.
	MaxPayloadBytes int64
	// PingIntervalSec is the keepalive ping cadence. The WritePump sends
	// a ping frame on this interval and expects a pong back.
	PingIntervalSec int
	// WriteTimeoutSec is the per-write deadline applied to the
	// underlying connection by the WritePump.
	WriteTimeoutSec int
	// ReadTimeoutSec is the per-read deadline applied by the ReadPump.
	ReadTimeoutSec int
	// OutBufferSize is the per-client Send channel depth.
	OutBufferSize int
}

// DefaultLimits returns the spec-mandated safe defaults. Values are
// chosen so a misconfigured deployment still behaves: keepalive is
// present, queues are bounded, and no cap is zero.
func DefaultLimits() Limits {
	return Limits{
		MaxConnections:     1000,
		MaxChannelsPerConn: 32,
		MaxMessageBytes:    64 * 1024,  // 64 KiB
		MaxPayloadBytes:    256 * 1024, // 256 KiB
		PingIntervalSec:    30,
		WriteTimeoutSec:    10,
		ReadTimeoutSec:     60,
		OutBufferSize:      DefaultOutBufferSize,
	}
}

// Validate enforces invariants that YAML/env alone cannot guarantee.
// All numeric fields must be strictly positive; OutBufferSize may be
// zero only when the caller intends to disable buffering (not used
// here, so we require > 0 too).
func (l Limits) Validate() error {
	switch {
	case l.MaxConnections <= 0:
		return errors.New("hub: MaxConnections must be > 0")
	case l.MaxChannelsPerConn <= 0:
		return errors.New("hub: MaxChannelsPerConn must be > 0")
	case l.MaxMessageBytes <= 0:
		return errors.New("hub: MaxMessageBytes must be > 0")
	case l.MaxPayloadBytes <= 0:
		return errors.New("hub: MaxPayloadBytes must be > 0")
	case l.PingIntervalSec <= 0:
		return errors.New("hub: PingIntervalSec must be > 0")
	case l.WriteTimeoutSec <= 0:
		return errors.New("hub: WriteTimeoutSec must be > 0")
	case l.ReadTimeoutSec <= 0:
		return errors.New("hub: ReadTimeoutSec must be > 0")
	case l.OutBufferSize <= 0:
		return errors.New("hub: OutBufferSize must be > 0")
	}
	return nil
}

// String renders the limits for diagnostic logging. It deliberately
// omits the value-by-value numeric so a single line is enough.
func (l Limits) String() string {
	return fmt.Sprintf(
		"Limits{conns=%d chans/conn=%d msg=%dB pub=%dB ping=%ds write=%ds read=%ds buf=%d}",
		l.MaxConnections, l.MaxChannelsPerConn, l.MaxMessageBytes, l.MaxPayloadBytes,
		l.PingIntervalSec, l.WriteTimeoutSec, l.ReadTimeoutSec, l.OutBufferSize,
	)
}
