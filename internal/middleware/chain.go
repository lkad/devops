package middleware

import (
	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Chain returns the canonical middleware stack in the order mandated by
// the middleware-stack spec:
//
//	Recovery → RequestID → CORS → Logger → (Auth → RBAC) → Handler
//
// Recovery is outermost so a panic anywhere downstream is caught.
// RequestID runs before CORS and Logger so the id is available to both
// the response header and the log line. Auth and RBAC are injected by
// their respective owners (Phase 2) and are not part of this function —
// callers append them after this chain in the route group where they
// apply.
//
// The function is safe to call with a nil logger (Logger becomes a
// no-op) and a nil/empty corsOrigins slice (CORS becomes a no-restrictions
// echo).
func Chain(log *logger.Logger, corsOrigins []string) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		Recovery(log),
		RequestID(),
		CORS(corsOrigins),
		Logger(log),
	}
}
