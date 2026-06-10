package hub

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// upgrader is the package-level *websocket.Upgrader used by the
// handler. CheckOrigin is permissive — auth happens via the JWT
// before the upgrade, so a CSRF-style origin check is unnecessary
// in this single-trust-domain deployment.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandlerConfig wires the runtime dependencies the handler needs.
// The struct is small because the hub is the only collaborator.
type HandlerConfig struct {
	Hub    *Hub
	Signer *auth.Signer
	// Upgrader is optional; nil falls back to the package default.
	// Tests inject a tighter upgrader when they want to assert on
	// the handshake.
	Upgrader *websocket.Upgrader
}

// RegisterRoutes installs the /ws upgrade route on router. The
// handler is the only public surface of this package; everything
// else is exercised via the Hub.
//
// The supplied permFactory is used to apply per-route RBAC to
// the upgrade path. Pass a no-op factory in tests that don't
// exercise auth. Pre-handler auth middleware (e.g. JWT
// verification on a non-WS path) is the caller's responsibility.
//
// This package implements the *terminal* handler for the
// upgrade path.
func RegisterRoutes(router gin.IRouter, h *Hub, signer *auth.Signer, up *websocket.Upgrader, permFactory func(rbac.Permission) gin.HandlerFunc) {
	cfg := HandlerConfig{Hub: h, Signer: signer, Upgrader: up}
	if router == nil || h == nil || signer == nil {
		return
	}
	if permFactory == nil {
		permFactory = func(rbac.Permission) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		}
	}
	router.GET("/ws", permFactory(rbac.PermissionViewAlerts), cfg.handleUpgrade)
}

// handleUpgrade is the Gin handler for the /ws endpoint. It runs
// after the auth middleware has populated c.Request with the
// verified user, but for the websocket-hub spec we re-verify here
// because the WebSocket upgrade path is the only place where the
// client may also pass the token via ?token= query (browsers
// can't set headers on the WebSocket handshake).
//
// Steps:
//  1. Verify JWT — 401 JSON on failure.
//  2. Verify the request really is a WebSocket upgrade — 400 on
//     failure (regular HTTP hitting /ws).
//  3. Upgrade.
//  4. Register with the hub; start read/write pumps.
//  5. Hand the connection over to the pumps; the request is done.
func (cfg HandlerConfig) handleUpgrade(c *gin.Context) {
	r := c.Request
	userID, err := Authenticate(r, cfg.Signer)
	if err != nil {
		apiErr, _ := err.(*contracts.APIError)
		handler.WriteError(c.Writer, apiErr)
		return
	}

	// gorilla checks Upgrade / Connection / Sec-WebSocket-* in
	// Upgrade(). Use that as the single source of truth.
	up := cfg.Upgrader
	if up == nil {
		up = &upgrader
	}
	conn, err := up.Upgrade(c.Writer, r, nil)
	if err != nil {
		// up.Upgrade already wrote a 400 to the response writer.
		// We only land here for unexpected wrapper-level errors.
		_ = err
		return
	}

	// Register with the hub. ErrLimitReached → 503.
	client := NewClient(conn, userID, nil, cfg.Hub.limits, cfg.Hub)
	if err := cfg.Hub.Register(client); err != nil {
		_ = conn.Close()
		if errors.Is(err, ErrLimitReached) {
			handler.WriteError(c.Writer, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "websocket hub at capacity",
			})
			return
		}
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to register client",
		})
		return
	}

	// Send a one-shot confirmation frame so the spec's "client
	// receives connection confirmation message" scenario passes.
	client.Send <- []byte(`{"type":"connected","client_id":"` + client.ID() + `"}`)

	// Start the pumps. They take ownership of the connection; the
	// request itself returns.
	pumpCtx, cancel := context.WithCancel(context.Background())
	go func() {
		client.WritePump(pumpCtx)
		cancel()
	}()
	go func() {
		client.ReadPump(pumpCtx, cfg.Hub.handleClientMessage, func(c *Client) {
			cfg.Hub.Unregister(c)
		})
		cancel()
	}()
}
