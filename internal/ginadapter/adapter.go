package ginfadapter

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTPHandlerFunc is a function that handles HTTP requests in the traditional net/http style
type HTTPHandlerFunc func(http.ResponseWriter, *http.Request)

// GinToHTTPHandler converts a net/http-style handler to a Gin handler
// It also injects path params from Gin into the request context so that mux.Vars(r) works
func GinToHTTPHandler(handler HTTPHandlerFunc, paramNames ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build mux-style vars map from Gin path params
		vars := make(map[string]string)
		for _, name := range paramNames {
			vars[name] = c.Param(name)
		}

		// Inject vars into request context with gorilla/mux's context key
		// mux.Vars uses the context key "_muxVars" (from gorilla/mux)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), muxContextKey("_muxVars"), vars),
		)

		handler(c.Writer, c.Request)
	}
}

// muxContextKey is a wrapper to ensure unique context keys
type muxContextKey string

// GinContextToRequestMiddleware wraps a Gin handler to inject gin.Context into request context
// This allows existing handlers that use r.Context() to access gin context values
func GinContextToRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Next()
	}
}

// RequestToGinHandler wraps a function that takes (http.ResponseWriter, *http.Request)
// to a Gin handler function
func RequestToGinHandler(handler HTTPHandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c.Writer, c.Request)
	}
}

// GinMiddleware converts a net/http-style middleware to a Gin middleware
func GinMiddleware(middleware func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	}
}
