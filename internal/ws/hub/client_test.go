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

// dialer dials a real WebSocket against the test server.
var dialer = websocket.DefaultDialer

// startTestWS upgrades the request and returns the *server-side* conn
// alongside the client-side conn. The server-side goroutine simply
// drains reads so the test cannot deadlock on backpressure.
func startTestWS(t *testing.T) (*httptest.Server, *websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	srvConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		srvConnCh <- c
		// Drain reads so the test doesn't deadlock on backpressure
		// when the client side closes the conn first.
		go func() {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()
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

// TestClient_WritePump_SendsToConn covers the spec: when something
// is pushed to Send, the WritePump forwards it to the underlying
// *websocket.Conn.
func TestClient_WritePump_SendsToConn(t *testing.T) {
	_, cliConn, srvConn := startTestWS(t)

	lim := DefaultLimits()
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.WritePump(ctx)

	want := []byte(`{"hello":"write-pump"}`)
	c.Send <- want

	_ = cliConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := cliConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestClient_WritePump_PingExitsOnCtxCancel exercises the spec
// keepalive path: WritePump returns when ctx is cancelled.
func TestClient_WritePump_PingExitsOnCtxCancel(t *testing.T) {
	lim := DefaultLimits()
	lim.PingIntervalSec = 1
	lim.WriteTimeoutSec = 1

	_, _, srvConn := startTestWS(t)
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		c.WritePump(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WritePump did not return after ctx cancellation")
	}
}

// TestClient_Close_StopsPumps: Close() is idempotent and terminates
// WritePump.
func TestClient_Close_StopsPumps(t *testing.T) {
	_, _, srvConn := startTestWS(t)
	lim := DefaultLimits()
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		c.WritePump(ctx)
		close(done)
	}()

	// Allow WritePump to enter its select loop.
	time.Sleep(20 * time.Millisecond)
	c.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WritePump did not return after Close")
	}
}

// TestClient_Close_Idempotent asserts Close is safe to call twice.
func TestClient_Close_Idempotent(t *testing.T) {
	_, _, srvConn := startTestWS(t)
	lim := DefaultLimits()
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)
	c.Close()
	c.Close() // must not panic
}

// TestClient_SendChannelBuffered confirms the spec: Send is a
// buffered channel whose capacity matches OutBufferSize.
func TestClient_SendChannelBuffered(t *testing.T) {
	_, _, srvConn := startTestWS(t)
	lim := DefaultLimits()
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)
	if got := cap(c.Send); got != lim.OutBufferSize {
		t.Errorf("cap(Send) = %d, want %d", got, lim.OutBufferSize)
	}
}

// TestClient_ReadPump_PeerClose exercises the spec note: the read
// pump must react to a peer-initiated close frame and call the
// onClose callback.
func TestClient_ReadPump_PeerClose(t *testing.T) {
	_, cliConn, srvConn := startServerAndDial(t)
	lim := DefaultLimits()
	c := NewClient(srvConn, "u-1", []string{}, lim, nil)

	var (
		mu             sync.Mutex
		onCloseFired   bool
		onMessageCalls int
	)
	onMessage := func(_ *Client, _ []byte) {
		mu.Lock()
		onMessageCalls++
		mu.Unlock()
	}
	onClose := func(_ *Client) {
		mu.Lock()
		onCloseFired = true
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		c.ReadPump(ctx, onMessage, onClose)
		close(done)
	}()

	// Let ReadPump enter its loop.
	time.Sleep(50 * time.Millisecond)
	// Send a close message from the peer (the client-side conn).
	_ = cliConn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second))

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ReadPump did not return after peer close")
	}
	mu.Lock()
	defer mu.Unlock()
	if !onCloseFired {
		t.Error("expected onClose to fire on peer close")
	}
	_ = atomic.LoadInt32 // keep atomic import alive for older toolchains
	_ = onMessageCalls
}
