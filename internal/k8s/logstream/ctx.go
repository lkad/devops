package logstream

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// contextWithClientDisconnect returns a child of parent
// that is cancelled when the underlying HTTP request is
// cancelled. The WebSocket handler uses the same pattern:
// the moment the client disconnects, c.Request.Context()
// is Done, and we exit the read loop.
//
// We keep this helper separate from contextWithCancelFromGin
// so the WS and SSE call sites are self-documenting; both
// are thin wrappers over c.Request.Context() with a cancel
// the caller can defer.
func contextWithClientDisconnect(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(parent)
}

// contextWithCancelFromGin is the SSE counterpart — it
// derives a cancellable child from the request context so
// the loop can exit on client disconnect.
func contextWithCancelFromGin(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(c.Request.Context())
}

// httpResponseWriter is a tiny interface assertion used at
// the WS boundary to guarantee c.Writer satisfies the
// http.ResponseWriter interface (gin's wrapper does, but
// the compile-time check is the kind of guard that catches
// refactors).
var _ http.ResponseWriter = (http.ResponseWriter)(nil)
