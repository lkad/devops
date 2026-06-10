package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

func init() {
	// Silence Gin's debug output during tests.
	gin.SetMode(gin.ReleaseMode)
}

// TestHandler_Upgrade_Success: a request with a valid Upgrade
// header and a valid JWT is upgraded to a WebSocket; the client
// receives the post-accept confirmation.
func TestHandler_Upgrade_Success(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	signer := newTestSigner(t)
	tok := issueTestToken(t, signer, &contracts.User{ID: "u-1", Username: "alice", Role: contracts.RoleOperator})

	router := gin.New()
	RegisterRoutes(router, h, signer, nil, rbac.NoopPermFactory())

	srv := httptest.NewServer(router)
	defer srv.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+tok)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	cli, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v (status=%v)", err, statusOf(resp))
	}
	defer cli.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("status = %d, want 101", resp.StatusCode)
	}
}

// TestHandler_Upgrade_RejectsMissingToken: a request to /ws with
// no Authorization header is rejected with 401 BEFORE the upgrade.
func TestHandler_Upgrade_RejectsMissingToken(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	signer := newTestSigner(t)

	router := gin.New()
	RegisterRoutes(router, h, signer, nil, rbac.NoopPermFactory())

	srv := httptest.NewServer(router)
	defer srv.Close()

	// Plain HTTP request — the server should not upgrade and should
	// return 401 JSON.
	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	var env contracts.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != contracts.CodeUnauthorized {
		t.Errorf("code = %v, want %v", env.Error.Code, contracts.CodeUnauthorized)
	}
}

// TestHandler_Upgrade_RejectsBadUpgrade: a non-WebSocket request
// (no Upgrade: websocket header) is rejected with 400. The spec
// says "regular HTTP to /ws" returns 400.
func TestHandler_Upgrade_RejectsBadUpgrade(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	signer := newTestSigner(t)
	tok := issueTestToken(t, signer, &contracts.User{ID: "u-1", Role: contracts.RoleOperator})

	router := gin.New()
	RegisterRoutes(router, h, signer, nil, rbac.NoopPermFactory())
	srv := httptest.NewServer(router)
	defer srv.Close()

	// Send a request with a valid token but no upgrade header.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/ws", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestHandler_Accept_ThenSubscribePublish: end-to-end check that
// after the upgrade the client can subscribe and receive a
// broadcast from the hub.
func TestHandler_Accept_ThenSubscribePublish(t *testing.T) {
	h := NewHub(HubConfig{Limits: DefaultLimits()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	defer h.Shutdown(time.Second)

	signer := newTestSigner(t)
	tok := issueTestToken(t, signer, &contracts.User{ID: "u-1", Role: contracts.RoleOperator})

	router := gin.New()
	RegisterRoutes(router, h, signer, nil, rbac.NoopPermFactory())
	srv := httptest.NewServer(router)
	defer srv.Close()

	header := http.Header{}
	header.Set("Authorization", "Bearer "+tok)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	cli, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close()

	// Drain the post-accept "connected" frame.
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, conn, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read connected: %v", err)
	}
	if !strings.Contains(string(conn), "connected") {
		t.Errorf("connected message = %q, want 'connected' substring", conn)
	}

	// Subscribe.
	if err := cli.WriteMessage(websocket.TextMessage, []byte(`{"action":"subscribe","channel":"log"}`)); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// Read acknowledgement.
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if !strings.Contains(string(msg), "ack") {
		t.Errorf("ack message = %q, want ack", msg)
	}

	// Hub publishes; client should receive.
	h.Publish("log", []byte(`{"channel":"log","type":"log_entry","data":{"message":"hi"}}`))

	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := cli.ReadMessage()
	if err != nil {
		t.Fatalf("read publish: %v", err)
	}
	if !strings.Contains(string(got), "hi") {
		t.Errorf("got %q, want 'hi' substring", got)
	}
}

func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

// unused import guard.
var _ = auth.SigningAlgorithm
