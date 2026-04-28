package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

// Middleware is the net/http style middleware (kept for backward compatibility)
func Middleware(cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for certain paths (except /api/auth/me which requires auth)
			path := r.URL.Path
			if cfg.DevBypass ||
				(strings.HasPrefix(path, "/api/auth/") && path != "/api/auth/me") ||
				path == "/health" ||
				path == "/metrics" ||
				path == "/" ||
				strings.HasPrefix(path, "/assets/") ||
				strings.HasPrefix(path, "/favicon") ||
				(strings.HasPrefix(path, "/api/") && r.Method == http.MethodOptions) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				apierror.Unauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				apierror.Unauthorized(w, "invalid authorization header format")
				return
			}

			tokenString := parts[1]

			// Parse and validate token
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil || !token.Valid {
				apierror.Unauthorized(w, "invalid or expired token")
				return
			}

			// Attach user to request context
			user := &User{
				Username:    claims.Username,
				Roles:       claims.Roles,
				Permissions: claims.Permissions,
			}
			ctx := context.WithValue(r.Context(), UserContextKey, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MiddlewareGin is the Gin-style middleware
func MiddlewareGin(cfg *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for certain paths
		path := c.Request.URL.Path
		authHeader := c.GetHeader("Authorization")

		// In dev bypass mode, always set a dev user
		if cfg.DevBypass {
			devUser := &User{
				Username:    cfg.DevUsername,
				Roles:       cfg.DevRoles,
				Permissions: []string{"read", "test-deploy"},
			}
			// If a token is provided, try to parse claims from it
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					claims := &Claims{}
					if token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
						return []byte(cfg.JWTSecret), nil
					}); err == nil && token.Valid {
						devUser.Username = claims.Username
						devUser.Roles = claims.Roles
						devUser.Permissions = claims.Permissions
					}
				}
			}
			c.Set("user", devUser)
			c.Next()
			return
		}

		// Normal auth path
		if (strings.HasPrefix(path, "/api/auth/") && path != "/api/auth/me") ||
			path == "/health" ||
			path == "/metrics" ||
			path == "/" ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/favicon") ||
			(strings.HasPrefix(path, "/api/") && c.Request.Method == http.MethodOptions) {
			c.Next()
			return
		}

		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenString := parts[1]

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		user := &User{
			Username:    claims.Username,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		}
		c.Set("user", user)

		c.Next()
	}
}

// GetUserFromContext retrieves the authenticated user from context
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return nil
}