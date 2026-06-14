package logstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// HandlerConfig tunes the HTTP layer. MaxBackpressureLines
// sets the per-subscriber buffer; the handler uses it to
// size the BackpressureSink that absorbs the gap between
// the streamer and the wire writer.
type HandlerConfig struct {
	// MaxBackpressureLines is the per-subscriber buffer.
	// Zero falls back to DefaultMaxBackpressureLines.
	MaxBackpressureLines int

	// WSReadDeadline is the per-message read deadline on the
	// WebSocket. Zero → 60 seconds. Tests that want a tight
	// loop can shorten this.
	WSReadDeadline time.Duration

	// WSWriteDeadline is the per-message write deadline.
	// Zero → 10 seconds.
	WSWriteDeadline time.Duration
}

// Handler is the HTTP / WebSocket / SSE surface for the K8s
// pod log streaming subsystem. It is intentionally thin —
// validation, orchestration, and the realtime fan-out all
// live in the service.
type Handler struct {
	svc *Service
	cfg HandlerConfig
	// upgrader is the gorilla/websocket upgrader. The
	// CheckOrigin default is permissive — the spec says the
	// endpoints live under the standard auth middleware, so
	// origin checks are not the handler's concern. Production
	// wiring can swap in a stricter CheckOrigin.
	upgrader websocket.Upgrader
}

// NewHandler builds a Handler from a Service and a config.
// Zero fields in cfg are filled with sensible defaults.
func NewHandler(svc *Service, cfg HandlerConfig) *Handler {
	if cfg.MaxBackpressureLines == 0 {
		cfg.MaxBackpressureLines = DefaultMaxBackpressureLines
	}
	if cfg.WSReadDeadline == 0 {
		cfg.WSReadDeadline = 60 * time.Second
	}
	if cfg.WSWriteDeadline == 0 {
		cfg.WSWriteDeadline = 10 * time.Second
	}
	return &Handler{
		svc: svc,
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// Register attaches the log-streaming routes to the supplied
// router group. The group is expected to live under /api/v1.
//
//	GET /api/v1/k8s/clusters/:id/pods/:namespace/:pod/logs/stream   (WebSocket)
//	GET /api/v1/k8s/clusters/:id/pods/:namespace/:pod/logs/sse      (Server-Sent Events)
//	GET /api/v1/k8s/clusters/:id/pods/:namespace/:pod/logs          (one-shot)
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewK8sPodLogs)
	// :clusterID (not :id) to match the param name used by the k8s
	// handler in internal/k8s/handler.go — gin refuses two
	// different param names at the same path slot
	// ("':id' conflicts with existing wildcard ':clusterID'").
	r.GET("/k8s/clusters/:clusterID/pods/:namespace/:pod/logs/stream", viewP, h.StreamWS)
	r.GET("/k8s/clusters/:clusterID/pods/:namespace/:pod/logs/sse", viewP, h.StreamSSE)
	r.GET("/k8s/clusters/:clusterID/pods/:namespace/:pod/logs", viewP, h.GetLogs)
}

// requestFromGin is the URL-shape → StreamRequest adapter
// shared by all three endpoints.
func (h *Handler) requestFromGin(c *gin.Context) (StreamRequest, *contracts.APIError) {
	req := StreamRequest{
		ClusterID: c.Param("clusterID"),
		Namespace: c.Param("namespace"),
		Pod:       c.Param("pod"),
		Container: c.Query("container"),
		Follow:    parseBool(c.Query("follow")),
	}
	if v := c.Query("tail"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return req, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "tail must be an integer",
			}
		}
		req.TailLines = n
	}
	if v := c.Query("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return req, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "since must be an RFC3339 timestamp",
			}
		}
		req.Since = t
	}
	return req, nil
}

// parseBool is a permissive "true"/"1" → true; everything
// else → false. We don't use strconv.ParseBool because the
// spec uses lowercase "true" exclusively; "1" is a courtesy.
func parseBool(s string) bool {
	return s == "true" || s == "1"
}

// StreamWS is the WebSocket endpoint. On accept, the handler
// runs Stream and pumps each line as a JSON frame to the
// client. The frame is a LogFrame so the routing fields
// (cluster/namespace/pod) are echoed back without a
// separate envelope.
func (h *Handler) StreamWS(c *gin.Context) {
	req, apiErr := h.requestFromGin(c)
	if apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	// Force Follow=true on the streaming endpoint — the
	// one-shot endpoint is /logs (no upgrade).
	req.Follow = true

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote a response; just return.
		return
	}
	defer conn.Close()

	ctx, cancel := contextWithClientDisconnect(c.Request.Context())
	defer cancel()

	ch, err := h.svc.Stream(ctx, req)
	if err != nil {
		// We can't write a standard error envelope after the
		// upgrade — the client expects WS frames. We send
		// a single "error" frame and close.
		_ = writeWSError(conn, err)
		return
	}
	conn.SetReadDeadline(time.Now().Add(h.cfg.WSReadDeadline))
	// Drain client-side pings / control frames so the
	// connection stays healthy. We don't process incoming
	// messages; the only client-side signal that matters is
	// the close.
	go h.drainClient(conn)

	sink := NewBackpressureSink(h.cfg.MaxBackpressureLines)
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			sink.Push(line)
			frame := LogFrame{
				ClusterID: req.ClusterID,
				Namespace: req.Namespace,
				Pod:       req.Pod,
				Container: req.Container,
				Line:      line,
				Dropped:   sink.Snapshot(),
			}
			conn.SetWriteDeadline(time.Now().Add(h.cfg.WSWriteDeadline))
			if err := conn.WriteJSON(frame); err != nil {
				return
			}
		}
	}
}

// writeWSError sends a single "error" frame so the client
// can surface the failure before we close. We use a typed
// payload (`type: "error"`) so a generic client can
// distinguish from data frames.
func writeWSError(conn *websocket.Conn, err error) error {
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		apiErr = &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: err.Error(),
		}
	}
	return conn.WriteJSON(gin.H{
		"type": "error",
		"error": gin.H{
			"code":    apiErr.Code,
			"message": apiErr.Message,
		},
	})
}

// drainClient reads incoming messages until the connection
// drops. We don't act on the messages — the only purpose
// is to keep the read pump alive so the upgrader observes
// disconnects promptly.
func (h *Handler) drainClient(conn *websocket.Conn) {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// StreamSSE is the Server-Sent Events endpoint. The wire
// format is `text/event-stream` with one event per line,
// framed as `data: <json>\n\n` per the SSE spec.
func (h *Handler) StreamSSE(c *gin.Context) {
	req, apiErr := h.requestFromGin(c)
	if apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	req.Follow = true

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "streaming unsupported by underlying writer",
		})
		return
	}

	ctx, cancel := contextWithCancelFromGin(c)
	defer cancel()

	ch, err := h.svc.Stream(ctx, req)
	if err != nil {
		// We've already written the 200 + headers. The best
		// we can do is send an error event so the client can
		// surface the failure.
		_, _ = fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			mustJSON(gin.H{"code": apiCode(err), "message": err.Error()}))
		flusher.Flush()
		return
	}
	sink := NewBackpressureSink(h.cfg.MaxBackpressureLines)
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			sink.Push(line)
			frame := LogFrame{
				ClusterID: req.ClusterID,
				Namespace: req.Namespace,
				Pod:       req.Pod,
				Container: req.Container,
				Line:      line,
				Dropped:   sink.Snapshot(),
			}
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", mustJSON(frame))
			flusher.Flush()
		}
	}
}

// GetLogs is the one-shot non-streaming endpoint. The body
// is the standard `{"data":[...]}` envelope.
func (h *Handler) GetLogs(c *gin.Context) {
	req, apiErr := h.requestFromGin(c)
	if apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	if req.TailLines == 0 {
		req.TailLines = DefaultMaxLines
	}
	// One-shot is always Follow=false; the request shape
	// doesn't change the service's contract, but the
	// behaviour is implicit.
	req.Follow = false
	lines, err := h.svc.GetHistorical(c.Request.Context(), req)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": lines})
}

// apiCode is a small helper that returns the wire code for
// an error, defaulting to INTERNAL_ERROR.
func apiCode(err error) contracts.ErrorCode {
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return contracts.CodeInternal
}

// mustJSON marshals v or panics — used for SSE event
// payloads where marshal failure is a programming error.
func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("logstream: mustJSON: %v", err))
	}
	return string(raw)
}

// writeAPIError is the same adapter used by the rest of the
// codebase: an arbitrary error → the standard ErrorResponse
// envelope.

// =============================================================================
// Log level inference. The k8s log stream is raw text — the
// kubelet doesn't tag lines with levels. The spec mandates
// that the streaming layer infer a level from the line's
// content so the downstream log-aggregation surface can
// filter by it.
//
// The heuristics are intentionally simple and case-
// insensitive: "ERROR", "FATAL" → error/fatal, "WARN" →
// warn, anything else → info. We substring-match (not
// regex) so the cost is trivial on a hot path.
// =============================================================================

// Level names match the spec's downstream contract (the
// /logs/query level filter).
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

// InferLevel walks the line looking for a level token. The
// order is fatal > error > warn so a "FATAL: ERROR …" line
// is classified as fatal (the most severe). The function is
// pure so callers can use it on hot paths.
func InferLevel(line string) string {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "FATAL") {
		return LevelFatal
	}
	if strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR ") ||
		strings.Contains(upper, "FAIL") {
		return LevelError
	}
	if strings.Contains(upper, "WARN") {
		return LevelWarn
	}
	return LevelInfo
}
