package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/devops-toolkit/pkg/errors"
	"github.com/devops-toolkit/pkg/response"
)

type contextKey string

const UserContextKey contextKey = "user"

func AuthMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, errors.Unauthorized("missing authorization header"))
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.Error(w, errors.Unauthorized("invalid authorization header format"))
				return
			}

			claims, err := jwtService.ValidateToken(parts[1])
			if err != nil {
				response.Error(w, errors.Unauthorized("invalid or expired token"))
				return
			}

			user := &User{
				Username: claims.Username,
				Email:    claims.Email,
				Roles:    claims.Roles,
				Group:    claims.Group,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return nil
}

func GetUsernameFromContext(ctx context.Context) string {
	if user := GetUserFromContext(ctx); user != nil {
		return user.Username
	}
	return ""
}

func HasRole(ctx context.Context, role string) bool {
	user := GetUserFromContext(ctx)
	if user == nil {
		return false
	}

	for _, r := range user.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func HasAnyRole(ctx context.Context, roles ...string) bool {
	for _, role := range roles {
		if HasRole(ctx, role) {
			return true
		}
	}
	return false
}
