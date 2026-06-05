package hub

import (
	"sort"
	"sync"
)

// ClientLike is the subset of *Client that the channel registry
// touches. Defining it as an interface keeps the registry unit-
// testable in isolation from the real *Client (which pulls in
// gorilla/websocket and a live conn).
type ClientLike interface {
	// ID uniquely identifies the client. Used as the map key in
	// per-channel subscriber sets.
	ID() string
	// SendChan is the buffered channel the WritePump reads from. Broadcast
	// is non-blocking; a full buffer means the client is slow and
	// the hub's queue-overflow disconnect path will reap it.
	SendChan() chan []byte
}

// ChannelRegistry owns the per-channel subscriber sets. All public
// methods are safe for concurrent use.
//
// Internal layout: outer map is channel-name → set of subscribers
// keyed by client ID. We use map[string]map[string]ClientLike rather
// than map[string][]ClientLike so Add/Remove are O(1) and Broadcast
// stays O(N) over the live subscriber set (not over disconnected
// ghosts).
type ChannelRegistry struct {
	mu     sync.RWMutex
	lookup map[string]map[string]ClientLike
	// rev lets RemoveAll find every channel a client is in without
	// scanning all channels. The map is keyed by client ID.
	rev map[string]map[string]struct{}
}

// NewChannelRegistry returns an empty registry.
func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{
		lookup: make(map[string]map[string]ClientLike),
		rev:    make(map[string]map[string]struct{}),
	}
}

// Add registers c as a subscriber of channel. Idempotent: adding
// the same client twice is a no-op (no error, no duplicate entry).
func (r *ChannelRegistry) Add(channel string, c ClientLike) {
	if channel == "" || c == nil {
		return
	}
	id := c.ID()
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.lookup[channel]
	if !ok {
		set = make(map[string]ClientLike)
		r.lookup[channel] = set
	}
	set[id] = c
	rev, ok := r.rev[id]
	if !ok {
		rev = make(map[string]struct{})
		r.rev[id] = rev
	}
	rev[channel] = struct{}{}
}

// Remove unregisters c from channel. Missing client or missing
// channel is a no-op.
func (r *ChannelRegistry) Remove(channel string, c ClientLike) {
	if channel == "" || c == nil {
		return
	}
	id := c.ID()
	r.mu.Lock()
	defer r.mu.Unlock()
	if set, ok := r.lookup[channel]; ok {
		delete(set, id)
		if len(set) == 0 {
			delete(r.lookup, channel)
		}
	}
	if rev, ok := r.rev[id]; ok {
		delete(rev, channel)
		if len(rev) == 0 {
			delete(r.rev, id)
		}
	}
}

// RemoveAll unregisters c from every channel it is currently in.
// Used on disconnect.
func (r *ChannelRegistry) RemoveAll(c ClientLike) {
	if c == nil {
		return
	}
	id := c.ID()
	r.mu.Lock()
	defer r.mu.Unlock()
	rev, ok := r.rev[id]
	if !ok {
		return
	}
	for ch := range rev {
		if set, ok := r.lookup[ch]; ok {
			delete(set, id)
			if len(set) == 0 {
				delete(r.lookup, ch)
			}
		}
	}
	delete(r.rev, id)
}

// Broadcast sends payload to every subscriber of channel. The send
// is non-blocking: a subscriber whose Send buffer is full is skipped
// and the hub's queue-overflow path is responsible for reaping it.
// This guarantees the hub never stalls because of one slow client.
func (r *ChannelRegistry) Broadcast(channel string, payload []byte) {
	r.mu.RLock()
	set, ok := r.lookup[channel]
	if !ok {
		r.mu.RUnlock()
		return
	}
	// Snapshot under read lock so we don't hold it across sends
	// (per-channel Send channels are independent and may block on
	// a different mutex if Send is upgraded later).
	subs := make([]ClientLike, 0, len(set))
	for _, c := range set {
		subs = append(subs, c)
	}
	r.mu.RUnlock()

	for _, c := range subs {
		select {
		case c.SendChan() <- payload:
		default:
			// Slow client: drop. The hub's Read/Write pump will
			// notice the Send buffer is full and disconnect.
		}
	}
}

// SubscriberCount returns the number of live subscribers of channel.
// Returns 0 for unknown channels.
func (r *ChannelRegistry) SubscriberCount(channel string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if set, ok := r.lookup[channel]; ok {
		return len(set)
	}
	return 0
}

// Channels returns the names of channels that currently have at
// least one subscriber. The result is sorted so tests are stable.
// Safe for concurrent use.
func (r *ChannelRegistry) Channels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.lookup))
	for name := range r.lookup {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
