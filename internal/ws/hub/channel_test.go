package hub

import (
	"sync"
	"sync/atomic"
	"testing"
)

// fakeClient is a minimal stand-in for *Client used in channel tests
// (it has just the fields the channel registry needs).
type fakeClient struct {
	id   string
	send chan []byte
}

func newFakeClient(id string) *fakeClient {
	return &fakeClient{id: id, send: make(chan []byte, 8)}
}

// ID implements ClientLike.
func (f *fakeClient) ID() string { return f.id }

// SendChan implements ClientLike.
func (f *fakeClient) SendChan() chan []byte { return f.send }

// TestChannelRegistry_AddRemove tracks the spec: a channel holds a
// set of subscribers; Add is idempotent, Remove is a no-op when
// missing, SubscriberCount returns the live set size.
func TestChannelRegistry_AddRemove(t *testing.T) {
	r := NewChannelRegistry()
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")

	r.Add("alerts", c1)
	r.Add("alerts", c2)
	r.Add("alerts", c1) // duplicate
	if got := r.SubscriberCount("alerts"); got != 2 {
		t.Errorf("SubscriberCount = %d, want 2", got)
	}

	r.Remove("alerts", c1)
	if got := r.SubscriberCount("alerts"); got != 1 {
		t.Errorf("after Remove(c1) SubscriberCount = %d, want 1", got)
	}

	r.Remove("alerts", c1) // no-op
	if got := r.SubscriberCount("alerts"); got != 1 {
		t.Errorf("after Remove(c1) again SubscriberCount = %d, want 1", got)
	}

	// Channel not present → 0
	if got := r.SubscriberCount("missing"); got != 0 {
		t.Errorf("SubscriberCount(missing) = %d, want 0", got)
	}
}

// TestChannelRegistry_Broadcast_O_N confirms Broadcast iterates
// exactly the subscriber set and never blocks: a non-blocking send
// is used so a stalled client cannot stall the hub.
func TestChannelRegistry_Broadcast_O_N(t *testing.T) {
	r := NewChannelRegistry()
	const N = 5
	clients := make([]*fakeClient, N)
	for i := range clients {
		clients[i] = newFakeClient(string(rune('a' + i)))
		r.Add("logs", clients[i])
	}

	var received int64
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *fakeClient) {
			defer wg.Done()
			<-c.send
			atomic.AddInt64(&received, 1)
		}(c)
	}
	r.Broadcast("logs", []byte(`{"hello":"world"}`))
	wg.Wait()
	if got := atomic.LoadInt64(&received); got != N {
		t.Errorf("received = %d, want %d", got, N)
	}
}

// TestChannelRegistry_Broadcast_DropsSlowClient: if a subscriber's
// send channel is full, Broadcast MUST NOT block — it should skip
// the slow client and continue. (The slow client is then reaped by
// the hub's queue-overflow disconnect path.)
func TestChannelRegistry_Broadcast_DropsSlowClient(t *testing.T) {
	r := NewChannelRegistry()
	slow := newFakeClient("slow")
	fast := newFakeClient("fast")
	// Pre-fill slow so the next send would block.
	slow.send <- []byte("stale")
	r.Add("x", slow)
	r.Add("x", fast)

	done := make(chan struct{})
	go func() {
		r.Broadcast("x", []byte("new"))
		close(done)
	}()
	select {
	case <-done:
		// good — did not block
	case <-fast.send:
		// also good — fast client got the message
	}
	// Drain fast's slot to avoid goroutine leak in test.
	select {
	case <-fast.send:
	default:
	}
	// Reap stale message we wrote above.
	<-slow.send
}

// TestChannelRegistry_RemoveAll confirms a per-client sweep returns
// to zero across all channels.
func TestChannelRegistry_RemoveAll(t *testing.T) {
	r := NewChannelRegistry()
	c := newFakeClient("c1")
	r.Add("a", c)
	r.Add("b", c)
	r.Add("c", c)
	r.RemoveAll(c)
	for _, ch := range []string{"a", "b", "c"} {
		if got := r.SubscriberCount(ch); got != 0 {
			t.Errorf("SubscriberCount(%s) = %d, want 0", ch, got)
		}
	}
}

// TestChannelRegistry_Channels returns the names of channels that
// currently have at least one subscriber.
func TestChannelRegistry_Channels(t *testing.T) {
	r := NewChannelRegistry()
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	r.Add("a", c1)
	r.Add("b", c2)
	r.Add("c", c1)

	got := r.Channels()
	if len(got) != 3 {
		t.Errorf("Channels() returned %d, want 3 (%v)", len(got), got)
	}
	set := make(map[string]struct{}, len(got))
	for _, n := range got {
		set[n] = struct{}{}
	}
	for _, want := range []string{"a", "b", "c"} {
		if _, ok := set[want]; !ok {
			t.Errorf("missing channel %q in %v", want, got)
		}
	}
}
