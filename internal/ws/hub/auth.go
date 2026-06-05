package hub

import (
	"errors"
	"net/http"
	"strings"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrMissingToken is returned when the request carries no credential
// at all. Callers (handler) map it to 401.
var ErrMissingToken = errors.New("hub: missing websocket auth token")

// Authenticate pulls a JWT from the request — first the
// `Authorization: Bearer` header, then the `?token=` query parameter
// as a fallback for browsers that can't set headers on a WebSocket
// handshake — and verifies it against the supplied Signer.
//
// Returns the user ID on success. On any verification failure
// (missing, malformed, wrong-secret, expired) returns a
// *contracts.APIError with Code = CodeUnauthorized so the HTTP
// handler can render the standard error envelope.
func Authenticate(r *http.Request, signer *auth.Signer) (string, error) {
	raw := extractToken(r)
	if raw == "" {
		return "", &contracts.APIError{
			Code:    contracts.CodeUnauthorized,
			Message: "missing websocket auth token",
		}
	}
	claims, err := signer.Verify(raw)
	if err != nil {
		return "", &contracts.APIError{
			Code:    contracts.CodeUnauthorized,
			Message: "invalid websocket auth token",
			Cause:   err,
		}
	}
	if claims == nil || claims.UserID == "" {
		return "", &contracts.APIError{
			Code:    contracts.CodeUnauthorized,
			Message: "websocket token has no subject",
		}
	}
	return claims.UserID, nil
}

// extractToken prefers Authorization: Bearer <tok>; if absent, falls
// back to ?token=<tok>. Empty string means "no token present" — the
// caller decides what that means (handler maps to 401).
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		// Case-insensitive scheme check; the spec mandates Bearer.
		if i := strings.Index(h, " "); i > 0 {
			scheme := strings.ToLower(strings.TrimSpace(h[:i]))
			if scheme != "bearer" {
				return "" // wrong scheme — treat as missing
			}
			return strings.TrimSpace(h[i+1:])
		}
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}
