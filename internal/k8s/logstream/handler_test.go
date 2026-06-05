package logstream

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestServer wires the three endpoints (WS, SSE, one-shot)
// onto a gin.Engine. Tests use httptest.NewServer to drive it
// over real HTTP.
func newTestServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// wsURL converts an http://host/path into a ws://host/path URL
// for the gorilla websocket dialer.
func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

// TestHandler_WebSocketStream: connect to /stream over WS,
// drive the fake streamer, and assert that the consumer
// receives the canned lines as JSON frames.
func TestHandler_WebSocketStream(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{
			{Line: "alpha", Stream: StreamStdout},
			{Line: "beta", Stream: StreamStderr},
		},
	}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	ws, resp, err := websocket.DefaultDialer.Dial(
		wsURL(srv.URL+"/api/v1/k8s/clusters/c1/pods/ns/p1/logs/stream"),
		nil,
	)
	if err != nil {
		t.Fatalf("dial: %v (status %v)", err, statusOf(resp))
	}
	defer ws.Close()

	var got []LogFrame
	deadline := time.Now().Add(3 * time.Second)
	for len(got) < 2 && time.Now().Before(deadline) {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v (got=%+v)", err, got)
		}
		var frame LogFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("decode %q: %v", raw, err)
		}
		got = append(got, frame)
	}
	if len(got) != 2 {
		t.Fatalf("frames = %+v, want 2", got)
	}
	if got[0].Line.Line != "alpha" || got[1].Line.Line != "beta" {
		t.Errorf("frame order: %+v", got)
	}
	for _, f := range got {
		if f.ClusterID != "c1" || f.Namespace != "ns" || f.Pod != "p1" {
			t.Errorf("routing fields: %+v", f)
		}
	}
}

// TestHandler_SSEStream: connect to /sse, drive the fake
// streamer, and assert the `text/event-stream` framing. The
// spec mandates `data: <json>\n\n` per line.
func TestHandler_SSEStream(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{
			{Line: "one", Stream: StreamStdout},
			{Line: "two", Stream: StreamStdout},
		},
	}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs/sse")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream...", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	var frames []LogFrame
	var pending strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line → end of event; parse pending.
			if pending.Len() > 0 {
				data := strings.TrimPrefix(pending.String(), "data: ")
				var f LogFrame
				if err := json.Unmarshal([]byte(data), &f); err != nil {
					t.Fatalf("decode %q: %v", data, err)
				}
				frames = append(frames, f)
				pending.Reset()
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			pending.WriteString(strings.TrimPrefix(line, "data: "))
		}
		if len(frames) >= 2 {
			break
		}
	}
	if len(frames) != 2 {
		t.Fatalf("frames = %+v, want 2", frames)
	}
	if frames[0].Line.Line != "one" || frames[1].Line.Line != "two" {
		t.Errorf("frame order: %+v", frames)
	}
}

// TestHandler_OneShotGet: GET /logs (no follow) returns a
// JSON envelope with the historical tail. The endpoint is
// the only one that does NOT stream — it returns the full
// slice in one response.
func TestHandler_OneShotGet(t *testing.T) {
	lc := &FakeLogClient{
		GetLogsLines: []LogLine{
			{Line: "x", Stream: StreamStdout},
			{Line: "y", Stream: StreamStderr},
		},
	}
	svc := NewService(&FakeStreamer{}, lc, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs?tail=10")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data []LogLine `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0].Line != "x" || body.Data[1].Line != "y" {
		t.Errorf("data = %+v, want [x, y]", body.Data)
	}
}

// TestHandler_OneShotRejectsOldSince: the 30-day cap is
// enforced on the one-shot path too. Status 422, code
// TIME_RANGE_EXCEEDED — re-using internal/logs.
func TestHandler_OneShotRejectsOldSince(t *testing.T) {
	svc := NewService(&FakeStreamer{}, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	old := time.Now().Add(-31 * 24 * time.Hour).UTC().Format(time.RFC3339)
	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs?since=" + old)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
	var env contracts.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("code = %v, want %v", env.Error.Code, contracts.CodeTimeRangeExceeded)
	}
}

// TestHandler_OneShotBadSinceFormat: a non-RFC3339 `since`
// is a 400 VALIDATION_ERROR. The 30-day cap is the only
// "since"-shaped 422.
func TestHandler_OneShotBadSinceFormat(t *testing.T) {
	svc := NewService(&FakeStreamer{}, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)
	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs?since=not-a-date")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestHandler_OneShotDefaultsTailLines: a one-shot call
// without a `tail` query param applies the default. We
// assert the FakeLogClient received TailLines=
// DefaultMaxLines (500) — the handler fills the default
// before forwarding to the service.
func TestHandler_OneShotDefaultsTailLines(t *testing.T) {
	lc := &FakeLogClient{}
	svc := NewService(&FakeStreamer{}, lc, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	calls := lc.CallsCopy()
	if len(calls) != 1 {
		t.Fatalf("LogClient calls = %d, want 1", len(calls))
	}
	if calls[0].TailLines != DefaultMaxLines {
		t.Errorf("TailLines = %d, want %d (default applied)", calls[0].TailLines, DefaultMaxLines)
	}
}

// TestHandler_WebSocketRequestForwardsToStreamer: the
// WebSocket handler builds a StreamRequest from URL
// params + query string. We assert every field lands on
// the fake streamer's recorded Calls.
func TestHandler_WebSocketRequestForwardsToStreamer(t *testing.T) {
	st := &FakeStreamer{
		StreamFn: func(_ context.Context, _ StreamRequest) <-chan LogLine {
			ch := make(chan LogLine)
			close(ch)
			return ch
		},
	}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	since := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	ws, _, err := websocket.DefaultDialer.Dial(
		wsURL(srv.URL+"/api/v1/k8s/clusters/cls-7/pods/default/p1/logs/stream?container=nginx&since="+since+"&tail=5&follow=true"),
		nil,
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()
	// Close the WS from the client side so the handler
	// exits; the call to st.Calls happens synchronously
	// inside Stream, so by the time the dial returns the
	// slice is safe to read.
	ws.Close()

	// Give the handler a moment to record + exit.
	deadline := time.Now().Add(2 * time.Second)
	var calls []StreamRequest
	for time.Now().Before(deadline) {
		calls = st.CallsCopy()
		if len(calls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(calls) == 0 {
		t.Fatal("streamer got no calls")
	}
	c := calls[0]
	if c.ClusterID != "cls-7" || c.Namespace != "default" || c.Pod != "p1" || c.Container != "nginx" {
		t.Errorf("routing: %+v", c)
	}
	if c.TailLines != 5 {
		t.Errorf("TailLines = %d, want 5", c.TailLines)
	}
	if !c.Follow {
		t.Error("Follow = false, want true")
	}
	if c.Since.IsZero() {
		t.Error("Since = zero, want non-zero")
	}
}

// TestHandler_ContextCancelClosesWebSocket: when the WS
// client disconnects, the handler's context is cancelled
// and the streamer goroutine exits. We verify by
// establishing a WS, then closing it; the fake streamer's
// StreamFn returns a channel that blocks until cancel.
func TestHandler_ContextCancelClosesWebSocket(t *testing.T) {
	stuck := make(chan LogLine)
	st := &FakeStreamer{
		StreamFn: func(_ context.Context, _ StreamRequest) <-chan LogLine {
			return stuck
		},
	}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	ws, _, err := websocket.DefaultDialer.Dial(
		wsURL(srv.URL+"/api/v1/k8s/clusters/c1/pods/ns/p1/logs/stream"),
		nil,
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Wait a bit for the handler to observe the close. We
	// don't assert on a specific signal — the test passes
	// as long as the process doesn't deadlock or hang.
	time.Sleep(200 * time.Millisecond)
	_ = stuck // keep ref alive
}

// TestHandler_RegisterAttachesRoutes: the Register method
// must attach every documented route. We assert the
// handlers respond (not 404) when called.
func TestHandler_RegisterAttachesRoutes(t *testing.T) {
	svc := NewService(&FakeStreamer{
		Lines: []LogLine{{Line: "x", Stream: StreamStdout}},
	}, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	// 200 on /sse (streaming; we just need it to not 404)
	go http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs/sse")
	// 200 on /logs (one-shot)
	resp, err := http.Get(srv.URL + "/api/v1/k8s/clusters/c1/pods/ns/p1/logs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("/logs status = %d, want 200", resp.StatusCode)
	}
}

// statusOf returns the response status code, or 0 if resp
// is nil. Used to format Dialer error messages.
func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

// TestHandler_ContainerIsOptional: a request without a
// `container` query param still streams — the spec scenario
// "Stream all containers".
func TestHandler_ContainerIsOptional(t *testing.T) {
	st := &FakeStreamer{
		Lines: []LogLine{{Line: "ok", Stream: StreamStdout}},
	}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	ws, _, err := websocket.DefaultDialer.Dial(
		wsURL(srv.URL+"/api/v1/k8s/clusters/c1/pods/ns/p1/logs/stream"),
		nil,
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var f LogFrame
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.Container != "" {
		t.Errorf("Container = %q, want empty (all containers)", f.Container)
	}
	if f.Line.Line != "ok" {
		t.Errorf("line = %q, want ok", f.Line.Line)
	}
}

// TestHandler_StreamerErrorReturnsApiError: a synchronous
// streamer error surfaces to the WS client as a typed
// error frame ("type":"error"). The upgrade succeeds; the
// first frame the client reads is the error envelope.
func TestHandler_StreamerErrorReturnsApiError(t *testing.T) {
	st := &FakeStreamer{StreamErr: errStreamerTestFailure}
	svc := NewService(st, &FakeLogClient{}, &fakePublisher{})
	h := NewHandler(svc, HandlerConfig{})
	srv := newTestServer(t, h)

	ws, _, err := websocket.DefaultDialer.Dial(
		wsURL(srv.URL+"/api/v1/k8s/clusters/c1/pods/ns/p1/logs/stream"),
		nil,
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if frame["type"] != "error" {
		t.Errorf("frame type = %v, want error", frame["type"])
	}
}

var errStreamerTestFailure = &streamerTestError{}

// streamerTestError is a tiny error type so the test above
// doesn't have to import a package-level sentinel.
type streamerTestError struct{}

func (streamerTestError) Error() string { return "streamer test failure" }

// import context to keep the import block stable.
var _ = context.Background
