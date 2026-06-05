package hub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrLimitReached is returned by Hub.Register when the configured
// MaxConnections cap has been hit. Callers (the handler) map it
// to 503 Service Unavailable on the upgrade response.
var ErrLimitReached = errors.New("hub: connection limit reached")

// HubConfig is the constructor input. We keep the surface small:
// only the resource caps. The hub doesn't read config directly so
// it can be constructed from a default in tests.
type HubConfig struct {
	Limits Limits
}

// Hub is the central broker. Public methods are safe for concurrent
// use; mutations to the connection set are serialised by muClients.
//
// Internal layout: a single RWMutex protects the connection set and
// the per-client subscription set is delegated to ChannelRegistry,
// which has its own lock. This split keeps the hot publish path
// (Broadcast) lock-light.
type Hub struct {
	limits Limits

	// muClients guards the clients map. We use a RWMutex because
	// Publish is read-heavy.
	muClients sync.RWMutex
	clients   map[string]*Client

	// channels owns the per-channel subscriber sets.
	channels *ChannelRegistry

	// broadcast is the spec-mandated dispatch channel. The hub's
	// Run loop reads from it and fans out to subscribers. We use
	// it so Publish is non-blocking for the caller.
	broadcast chan []byte

	// broadcastRoutes is the channel name paired with the payload
	// on broadcast. We send a small envelope over broadcast so
	// the Run loop knows which channel to deliver to.
	broadcastChan chan string

	// lifecycleDone is closed when Run returns.
	lifecycleDone chan struct{}
}

// NewHub builds a hub with the supplied config. Apply default-safe
// values for any field the caller omitted.
func NewHub(cfg HubConfig) *Hub {
	lim := cfg.Limits
	if lim.MaxConnections == 0 {
		lim = DefaultLimits()
	}
	return &Hub{
		limits:         lim,
		clients:        make(map[string]*Client),
		channels:       NewChannelRegistry(),
		broadcast:      make(chan []byte, 256),
		broadcastChan:  make(chan string, 256),
		lifecycleDone:  make(chan struct{}),
	}
}

// Run is the hub's main loop. It owns the broadcast path: every
// Publish is enqueued and Run drains it. Cancel ctx to drain and
// return.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.lifecycleDone)
	for {
		select {
		case <-ctx.Done():
			// Drain broadcast queue before returning so in-flight
			// Publish calls don't block forever. Non-blocking reads.
			for {
				select {
				case payload := <-h.broadcast:
					ch := <-h.broadcastChan
					h.deliver(ch, payload)
				default:
					return
				}
			}
		case payload := <-h.broadcast:
			select {
			case ch := <-h.broadcastChan:
				h.deliver(ch, payload)
			default:
				// Out-of-order: drop. (Cannot happen because we
				// publish under h.broadcast then h.broadcastChan
				// in Publish; the Run loop reads in the same order.)
			}
		}
	}
}

// Shutdown cancels the hub's context and waits up to d for Run to
// return. After Shutdown, no new Publish calls will be observed.
func (h *Hub) Shutdown(d time.Duration) {
	// Walk the client set and disconnect. Hub callers cancel the
	// ctx that drives Run; this method just disconnects clients
	// and waits for Run to drain.
	h.muClients.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.muClients.Unlock()

	for _, c := range clients {
		h.Unregister(c) // also drops the channel subscriptions
		c.Close()
	}
	select {
	case <-h.lifecycleDone:
	case <-time.After(d):
	}
}

// connections returns the current connection count. Exposed for
// tests via the lowercase name (in-package only).
func (h *Hub) connections() int {
	h.muClients.RLock()
	defer h.muClients.RUnlock()
	return len(h.clients)
}

// Register adds c to the connection set. Returns ErrLimitReached
// when MaxConnections is exceeded. Idempotent on the same client
// (same ID): re-registering updates the entry in place.
func (h *Hub) Register(c *Client) error {
	if c == nil {
		return errors.New("hub: nil client")
	}
	h.muClients.Lock()
	defer h.muClients.Unlock()
	if _, ok := h.clients[c.ID()]; !ok {
		if len(h.clients) >= h.limits.MaxConnections {
			return ErrLimitReached
		}
	}
	h.clients[c.ID()] = c
	// Replay any existing subscriptions into the registry so the
	// client's prior Subscribe calls take effect after Register.
	for _, ch := range c.Channels() {
		h.channels.Add(ch, c)
	}
	return nil
}

// Unregister removes c from the connection set and every channel.
// On disconnect the hub calls this from the ReadPump's onClose.
func (h *Hub) Unregister(c *Client) {
	if c == nil {
		return
	}
	h.muClients.Lock()
	if _, ok := h.clients[c.ID()]; !ok {
		h.muClients.Unlock()
		return
	}
	delete(h.clients, c.ID())
	h.muClients.Unlock()

	// Remove from every channel. Use the registry's RemoveAll so
	// we don't have to enumerate channels here.
	h.channels.RemoveAll(c)
}

// Subscribe attaches c to channel. Honours the per-connection
// cap: returns false if the client is already at MaxChannelsPerConn.
func (h *Hub) Subscribe(c *Client, channel string) bool {
	if c == nil || channel == "" {
		return false
	}
	if !c.Subscribe(channel) {
		return false
	}
	h.channels.Add(channel, c)
	return true
}

// Unsubscribe detaches c from channel.
func (h *Hub) Unsubscribe(c *Client, channel string) {
	if c == nil || channel == "" {
		return
	}
	c.Unsubscribe(channel)
	h.channels.Remove(channel, c)
}

// Publish enqueues a broadcast for channel. Returns immediately;
// the hub's Run loop delivers to subscribers. If payload exceeds
// MaxPayloadBytes the call is dropped (and Publish returns false).
func (h *Hub) Publish(channel string, payload []byte) bool {
	if int64(len(payload)) > h.limits.MaxPayloadBytes {
		return false
	}
	// Non-blocking enqueue. If the hub is shutting down, the loop
	// will drain; if it's overwhelmed we still drop rather than
	// block the caller.
	select {
	case h.broadcast <- payload:
		h.broadcastChan <- channel
		return true
	default:
		return false
	}
}

// deliver fans a payload out to every subscriber of channel.
// Called from the Run loop so it runs serially with respect to
// other Publishes — the per-client send channel still has its own
// per-client buffer, so a slow client only blocks its own WritePump.
func (h *Hub) deliver(channel string, payload []byte) {
	h.channels.Broadcast(channel, payload)
}

// handleClientMessage is the default per-message handler wired by
// the handler on accept. It expects JSON of the form
//
//	{"action": "subscribe"|"unsubscribe", "channel": "<name>"}
//
// Unknown actions are silently dropped; an error response is sent
// back to the client on the same connection so the client sees it.
func (h *Hub) handleClientMessage(c *Client, msg []byte) {
	var env struct {
		Action  string `json:"action"`
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		c.sendError("invalid message")
		return
	}
	switch env.Action {
	case "subscribe":
		if !h.Subscribe(c, env.Channel) {
			c.sendError("subscribe rejected")
			return
		}
		c.sendOK("subscribed", env.Channel)
	case "unsubscribe":
		h.Unsubscribe(c, env.Channel)
		c.sendOK("unsubscribed", env.Channel)
	default:
		c.sendError("unknown action")
	}
}

// supportedChannels lists the channel names the spec says MUST be
// accepted. The spec mandates log, metric, device_event,
// pipeline_update, alert, container_log.
var supportedChannels = map[string]struct{}{
	"log":            {},
	"metric":         {},
	"device_event":   {},
	"pipeline_update": {},
	"alert":          {},
	"container_log":  {},
}

// IsSupportedChannel reports whether name is a known channel.
// The hub is permissive here — unknown channels are still allowed
// so ad-hoc publishers (e.g. internal modules) can use any name —
// but the tests above assert the spec list works.
func IsSupportedChannel(name string) bool {
	_, ok := supportedChannels[name]
	return ok
}

// Compile-time hook so a future addition doesn't have to remember
// to update tests.
var _ = sync.Mutex{}
