package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// upgraderForTest returns a permissive upgrader for httptest servers.
func upgraderForTest() websocket.Upgrader {
	return websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
}

// startServerAndDial starts an httptest server that upgrades the
// request, returns the *server-side* conn (held by the test) and
// the *client-side* conn (held by the test). The server side does
// NOT run a drain goroutine — the test is expected to drive both
// pumps explicitly. This is the only safe way to use a single
// *websocket.Conn from multiple goroutines.
func startServerAndDial(t *testing.T) (*httptest.Server, *websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgrader := upgraderForTest()
	srvConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		srvConnCh <- c
		// Block the handler goroutine until the test cleans up so
		// the server doesn't return from ServeHTTP and race with
		// reads on c. We park on a per-server done channel.
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	cliConn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { cliConn.Close() })

	var srvConn *websocket.Conn
	select {
	case srvConn = <-srvConnCh:
	case <-time.After(2 * time.Second):
		t.Fatal("server-side conn never arrived")
	}
	return srv, cliConn, srvConn
}

// helper: read text frame with deadline.
func readText(t *testing.T, cli *websocket.Conn, d time.Duration) string {
	t.Helper()
	_ = cli.SetReadDeadline(time.Now().Add(d))
	_, msg, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(msg)
}

// TestHub_Register_Publish_Broadcast_Unsubscribe is the full
// integration scenario from the spec: two clients subscribe to a
// channel, a Publish reaches both, an Unsubscribe stops delivery
// to one.
func TestHub_Register_Publish_Broadcast_Unsubscribe(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	// Client 1
	_, cli1, srvConn1 := startServerAndDial(t)
	c1 := NewClient(srvConn1, "u-1", nil, DefaultLimits(), h)
	if err := h.Register(c1); err != nil {
		t.Fatalf("Register 1: %v", err)
	}
	pumpCtx1, pumpCancel1 := context.WithCancel(ctx)
	defer pumpCancel1()
	go c1.WritePump(pumpCtx1)
	go c1.ReadPump(pumpCtx1, h.handleClientMessage, func(client *Client) { h.Unregister(client) })

	// Client 2
	_, cli2, srvConn2 := startServerAndDial(t)
	c2 := NewClient(srvConn2, "u-2", nil, DefaultLimits(), h)
	if err := h.Register(c2); err != nil {
		t.Fatalf("Register 2: %v", err)
	}
	pumpCtx2, pumpCancel2 := context.WithCancel(ctx)
	defer pumpCancel2()
	go c2.WritePump(pumpCtx2)
	go c2.ReadPump(pumpCtx2, h.handleClientMessage, func(client *Client) { h.Unregister(client) })

	// Both subscribe to "log".
	h.Subscribe(c1, "log")
	h.Subscribe(c2, "log")

	// Publish a message; both clients should receive.
	if !h.Publish("log", []byte(`{"channel":"log","type":"log_entry","data":{"message":"hello"}}`)) {
		t.Fatal("Publish log returned false")
	}
	got1 := readText(t, cli1, 2*time.Second)
	got2 := readText(t, cli2, 2*time.Second)
	if !strings.Contains(got1, "hello") {
		t.Errorf("c1 got %q, want substring 'hello'", got1)
	}
	if !strings.Contains(got2, "hello") {
		t.Errorf("c2 got %q, want substring 'hello'", got2)
	}

	// Unsubscribe c2 from log.
	h.Unsubscribe(c2, "log")

	h.Publish("log", []byte(`{"channel":"log","type":"log_entry","data":{"message":"after-unsub"}}`))
	got1b := readText(t, cli1, 2*time.Second)
	if !strings.Contains(got1b, "after-unsub") {
		t.Errorf("c1 (still subscribed) got %q, want 'after-unsub'", got1b)
	}
	// c2 must NOT receive. Read with a short deadline and expect an
	// i/o timeout. We don't fail on the error string — only on a
	// non-error return.
	_ = cli2.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, _, err := cli2.ReadMessage()
	if err == nil {
		t.Errorf("c2 received a message after unsubscribe, expected none")
	}
}

// TestHub_Register_RejectsAtLimit: when MaxConnections is reached,
// the next registration returns ErrLimitReached.
func TestHub_Register_RejectsAtLimit(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxConnections = 1
	h := NewHub(HubConfig{Limits: lim})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	_, _, srvConn := startServerAndDial(t)
	c1 := NewClient(srvConn, "u-1", nil, lim, h)
	if err := h.Register(c1); err != nil {
		t.Fatalf("Register 1: %v", err)
	}
	c2 := NewClient(srvConn, "u-2", nil, lim, h)
	err := h.Register(c2)
	if err != ErrLimitReached {
		t.Errorf("Register 2 error = %v, want %v", err, ErrLimitReached)
	}
}

// TestHub_ConcurrentSubscribe: N goroutines subscribe distinct
// channels to the same client; the hub must record all of them
// without races.
func TestHub_ConcurrentSubscribe(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxChannelsPerConn = 100 // raise cap so all 50 fit
	h := NewHub(HubConfig{Limits: lim})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	_, _, srvConn := startServerAndDial(t)
	c := NewClient(srvConn, "u", nil, lim, h)
	if err := h.Register(c); err != nil {
		t.Fatalf("Register: %v", err)
	}

	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h.Subscribe(c, "ch-"+itoaForTest(i))
		}(i)
	}
	wg.Wait()
	for i := 0; i < N; i++ {
		if got := h.channels.SubscriberCount("ch-" + itoaForTest(i)); got != 1 {
			t.Errorf("SubscriberCount(ch-%d) = %d, want 1", i, got)
		}
	}
}

// itoaForTest is a test-local int-to-string.
func itoaForTest(n int) string {
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

// TestHub_UnregisterRemovesFromAllChannels: when a client is
// unregistered, every channel subscription is dropped.
func TestHub_UnregisterRemovesFromAllChannels(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	c := NewClient(nil, "u", nil, DefaultLimits(), h)
	h.Subscribe(c, "a")
	h.Subscribe(c, "b")
	if err := h.Register(c); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if h.connections() != 1 {
		t.Errorf("connections = %d, want 1", h.connections())
	}
	h.Unregister(c)
	if h.connections() != 0 {
		t.Errorf("after Unregister connections = %d, want 0", h.connections())
	}
	if h.channels.SubscriberCount("a") != 0 {
		t.Errorf("ch a still has subscribers after unregister")
	}
	if h.channels.SubscriberCount("b") != 0 {
		t.Errorf("ch b still has subscribers after unregister")
	}
}

// TestHub_Shutdown_DisconnectsAllClients asserts the graceful
// shutdown path drains the connection set.
func TestHub_Shutdown_DisconnectsAllClients(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	// Build three real clients.
	clients := make([]*Client, 0, 3)
	srvConns := make([]*websocket.Conn, 0, 3)
	for i := 0; i < 3; i++ {
		_, _, s := startServerAndDial(t)
		c := NewClient(s, "u-"+itoaForTest(i), nil, DefaultLimits(), h)
		if err := h.Register(c); err != nil {
			t.Fatalf("Register %d: %v", i, err)
		}
		clients = append(clients, c)
		srvConns = append(srvConns, s)
	}
	if h.connections() != 3 {
		t.Errorf("connections = %d, want 3", h.connections())
	}
	cancel()
	h.Shutdown(time.Second)
	if h.connections() != 0 {
		t.Errorf("after Shutdown connections = %d, want 0", h.connections())
	}
	for _, c := range clients {
		select {
		case <-c.Done():
		default:
			t.Errorf("client not closed after Shutdown")
		}
	}
	_ = srvConns
}

// Compile-time assertions for unused imports.
var _ = atomic.LoadInt32
