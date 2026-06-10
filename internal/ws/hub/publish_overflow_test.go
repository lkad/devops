package hub

import (
	"context"
	"testing"
	"time"
)

// TestHub_Publish_OverflowReturnsFalse pins the
// non-blocking contract: when the broadcast buffer is
// saturated (no Run consumer), Publish returns false
// instead of blocking the caller. The pre-fix bug sent
// on broadcastChan UNCONDITIONALLY (no select), so a
// saturated channel buffer would block the publisher
// forever — a denial-of-service surface for any caller
// that pushes events (the audit's physical-host monitor
// loop, the alert dispatcher, the audit emitter).
func TestHub_Publish_OverflowReturnsFalse(t *testing.T) {
	// broadcast + broadcastChan buffers are 256 each
	// (see Hub.NewHub). Without Run draining, the first
	// 256 publishes fill the buffer and the 257th
	// returns false. The pre-fix code would block on
	// the unconditional second send once the broadcast
	// buffer was full; the fix returns false instead.
	limits := DefaultLimits()
	limits.OutBufferSize = 1
	h := NewHub(HubConfig{Limits: limits})
	// NOTE: no h.Run here — that's the whole point.
	//
	// Fill the broadcast + broadcastChan buffers.
	for i := 0; i < 256; i++ {
		if ok := h.Publish("ch1", []byte("fill")); !ok {
			t.Fatalf("publish %d/256 should have succeeded, got false", i+1)
		}
	}
	// 257th publish: broadcast full. The pre-fix code
	// would block on the unconditional channel send.
	// The fix puts the outer send in a select with
	// default, so this returns false in microseconds.
	done := make(chan bool, 1)
	go func() {
		done <- h.Publish("ch1", []byte("overflow"))
	}()
	select {
	case got := <-done:
		if got {
			t.Errorf("257th publish: got true, want false (buffer full; non-blocking should drop)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked for 2s — the TOCTOU bug is back (unconditional channel send)")
	}
	_ = context.Background
}
