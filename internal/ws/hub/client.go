package hub

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection. The struct is
// shared between ReadPump and WritePump, both of which run as
// goroutines started by the handler on accept.
//
// Mu guards the per-client subscription set so the ReadPump (which
// mutates it on subscribe/unsubscribe) and the hub (which iterates
// it during Broadcast cleanup) do not race.
type Client struct {
	id  string
	uid string

	// Conn is the underlying gorilla *websocket.Conn. Only the
	// ReadPump and WritePump touch it; the handler does not.
	Conn *websocket.Conn

	// Send is the buffered channel the WritePump reads from.
	// Capacity is Limits.OutBufferSize. When it fills up the
	// hub disconnects the client (queue overflow).
	Send chan []byte

	// channels is the per-client subscription set, mutated by
	// the ReadPump on subscribe/unsubscribe. Mu guards it.
	Mu       sync.RWMutex
	channels map[string]struct{}

	// limits snapshots the resource caps at construction time so
	// the pumps don't have to ask the hub for them on every tick.
	limits Limits
	// onMessage/onClose are the per-ReadPump callbacks supplied by
	// the hub. They run synchronously on the ReadPump goroutine.
	onMessage func(c *Client, msg []byte)
	onClose   func(c *Client)

	// closeOnce guards Close so both pumps may call it without
	// double-closing the underlying connection.
	closeOnce sync.Once
	closed    chan struct{}
}

// NewClient constructs a Client and returns it ready to be
// registered with the hub. The hub argument is optional — when nil,
// the client's per-pump callbacks default to no-ops, which is
// useful in isolation tests.
func NewClient(conn *websocket.Conn, userID string, initial []string, lim Limits, h *Hub) *Client {
	if lim.OutBufferSize <= 0 {
		lim.OutBufferSize = DefaultOutBufferSize
	}
	c := &Client{
		id:       newClientID(),
		uid:      userID,
		Conn:     conn,
		Send:     make(chan []byte, lim.OutBufferSize),
		channels: make(map[string]struct{}, len(initial)),
		limits:   lim,
		closed:   make(chan struct{}),
	}
	for _, ch := range initial {
		c.channels[ch] = struct{}{}
	}
	// Default callbacks: no-op. The handler wires real ones via
	// SetHandlers. This split keeps NewClient usable from tests
	// that don't want to spin up a hub.
	if h != nil {
		c.onMessage = h.handleClientMessage
		c.onClose = func(client *Client) { h.Unregister(client) }
	}
	return c
}

// SetHandlers wires the per-message / per-close callbacks used by
// ReadPump. Call before starting the pumps. The handler calls this
// so the spec's "starts read/write goroutines" step is honoured.
func (c *Client) SetHandlers(onMessage func(c *Client, msg []byte), onClose func(c *Client)) {
	c.onMessage = onMessage
	c.onClose = onClose
}

// ID returns the unique client ID. Implements ClientLike.
func (c *Client) ID() string { return c.id }

// UserID returns the authenticated user that owns this client.
func (c *Client) UserID() string { return c.uid }

// SendChan returns the Send channel. Implements ClientLike.
func (c *Client) SendChan() chan []byte { return c.Send }

// Subscribe adds channel to the client's subscription set. Returns
// false if the per-client cap would be exceeded.
func (c *Client) Subscribe(channel string) bool {
	if channel == "" {
		return false
	}
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if _, ok := c.channels[channel]; ok {
		return true
	}
	if len(c.channels) >= c.limits.MaxChannelsPerConn {
		return false
	}
	c.channels[channel] = struct{}{}
	return true
}

// sendError pushes a JSON error envelope to the client. Best-effort:
// if Send is full the message is dropped; the queue-overflow path
// will reap the slow client.
func (c *Client) sendError(reason string) {
	env := map[string]any{"type": "error", "message": reason}
	b, _ := jsonMarshal(env)
	select {
	case c.Send <- b:
	default:
	}
}

// sendOK pushes a JSON acknowledgement.
func (c *Client) sendOK(reason, channel string) {
	env := map[string]any{"type": "ack", "status": reason, "channel": channel}
	b, _ := jsonMarshal(env)
	select {
	case c.Send <- b:
	default:
	}
}

// jsonMarshal is a tiny shim so client.go doesn't import encoding/json
// twice (it's a hot path on the ReadPump).
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unsubscribe removes channel from the client's subscription set.
// Idempotent.
func (c *Client) Unsubscribe(channel string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	delete(c.channels, channel)
}

// Channels returns a snapshot of the client's subscriptions.
func (c *Client) Channels() []string {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	out := make([]string, 0, len(c.channels))
	for ch := range c.channels {
		out = append(out, ch)
	}
	return out
}

// Close terminates the client. Idempotent. The hub's queue-
// overflow path and the per-pump error paths both call this.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}

// Done returns a channel that is closed when Close is called. Pumps
// use it as a non-ctx-dependent shutdown signal.
func (c *Client) Done() <-chan struct{} { return c.closed }

// ReadPump reads messages from the connection and dispatches them
// to onMessage. On error or close it invokes onClose and returns.
// ctx cancellation is a no-op for the read loop (it cannot cancel
// a blocked gorilla read); Done() is the primary shutdown signal.
func (c *Client) ReadPump(ctx context.Context, onMessage func(c *Client, msg []byte), onClose func(c *Client)) {
	if onMessage != nil {
		c.onMessage = onMessage
	}
	if onClose != nil {
		c.onClose = onClose
	}
	// Apply read deadline and message size limit from config.
	c.Conn.SetReadLimit(c.limits.MaxMessageBytes)
	_ = c.Conn.SetReadDeadline(time.Now().Add(time.Duration(c.limits.ReadTimeoutSec) * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(time.Duration(c.limits.ReadTimeoutSec) * time.Second))
		return nil
	})

	defer func() {
		c.Close()
		if c.onClose != nil {
			c.onClose(c)
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-c.closed:
			return
		default:
		}
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.Conn.SetReadDeadline(time.Now().Add(time.Duration(c.limits.ReadTimeoutSec) * time.Second))
		if c.onMessage != nil {
			c.onMessage(c, msg)
		}
	}
}

// WritePump drains Send and writes each frame to the connection.
// On a periodic tick (PingIntervalSec) it sends a ping frame and
// expects a pong back via ReadPump's SetPongHandler. Returns when
// ctx is cancelled, Send is closed, or Close is called.
func (c *Client) WritePump(ctx context.Context) {
	pingTicker := time.NewTicker(time.Duration(c.limits.PingIntervalSec) * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(time.Duration(c.limits.WriteTimeoutSec) * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(time.Duration(c.limits.WriteTimeoutSec) * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// newClientID returns a fresh unique ID. We use the wall clock plus
// a process-wide counter so IDs are sortable and don't collide even
// in tests that build many clients back-to-back.
var idCounter uint64

func newClientID() string {
	// Lightweight, dependency-free ID. Sufficient for a process-
	// local identifier; the spec doesn't require UUID.
	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + itoa(int(atomicAdd(&idCounter)))
}

// itoa is a local helper to avoid pulling in strconv for a single
// use; the ID is never user-visible and never parsed.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}

// atomicAdd is the smallest possible shim over sync/atomic so the
// file's imports stay minimal. Other files in the package are free
// to use sync/atomic directly.
func atomicAdd(p *uint64) uint64 {
	return atomicAdd64(p, 1)
}
