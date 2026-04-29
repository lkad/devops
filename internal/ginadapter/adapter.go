package ginfadapter

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
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

		// Store vars in request context under a string key
		// Vars() helper retrieves from here if mux.Vars returns nil
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), pathVarsKey, vars),
		)

		handler(c.Writer, c.Request)
	}
}

// pathVarsKey is the context key for path variables (string key for our storage)
var pathVarsKey = "path_vars"

// Vars retrieves path variables from the request.
// It first tries mux.Vars (for requests that went through gorilla/mux directly),
// then falls back to our Gin-stored vars (for requests through GinToHTTPHandler).
// This allows handlers to use Vars(r)["id"] regardless of the router used.
func Vars(r *http.Request) map[string]string {
	// First try mux.Vars (works for gorilla/mux routed requests)
	if vars := mux.Vars(r); vars != nil {
		return vars
	}
	// Fall back to our Gin-stored vars
	if vars := r.Context().Value(pathVarsKey); vars != nil {
		return vars.(map[string]string)
	}
	return nil
}

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
