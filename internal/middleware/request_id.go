package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey is the gin.Context key under which the request id is stored.
// Downstream handlers and the logger middleware both read from this key.
const RequestIDKey = "request_id"

// HeaderRequestID is the canonical request id header (request and response).
const HeaderRequestID = "X-Request-ID"

// RequestID returns a Gin middleware that ensures every request has an
// id. If the incoming request already carries an X-Request-ID header
// (e.g. from an upstream proxy or load balancer), it is propagated
// unchanged. Otherwise a new UUID is generated.
//
// The id is written back into the response header so the client can
// correlate logs, and it is stored in gin.Context under RequestIDKey
// so handlers and the logger middleware can read it.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}
