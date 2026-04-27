package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/devops-toolkit/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userContextKey contextKey = "user"

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
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
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
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Attach user to request context
			user := &User{
				Username:    claims.Username,
				Roles:       claims.Roles,
				Permissions: claims.Permissions,
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves the authenticated user from context
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}