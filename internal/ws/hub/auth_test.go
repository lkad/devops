package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newTestSigner returns a Signer with a fixed secret and a generous
// TTL so tests don't trip on expiration.
func newTestSigner(t *testing.T) *auth.Signer {
	t.Helper()
	s, err := auth.NewSigner("test-secret-do-not-reuse", time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

// issueTestToken mints a valid JWT for the supplied user.
func issueTestToken(t *testing.T, s *auth.Signer, u *contracts.User) string {
	t.Helper()
	tok, _, err := s.Issue(u)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return tok
}

// mintRawJWT builds an HS256 token with explicit claims so the test
// can place exp in the past regardless of Signer.ttl clamping.
func mintRawJWT(t *testing.T, secret, uid, usr string, role contracts.Role, exp int64) string {
	t.Helper()
	// lazy import via top of file (avoids adding more imports here)
	tok := buildHS256(t, secret, uid, usr, role, exp)
	return tok
}

// TestAuthenticate_BearerHeader confirms the happy path: a request
// with a valid Authorization: Bearer header returns the user ID and
// no error.
func TestAuthenticate_BearerHeader(t *testing.T) {
	s := newTestSigner(t)
	tok := issueTestToken(t, s, &contracts.User{ID: "u-1", Username: "alice", Role: contracts.RoleOperator})

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	gotID, err := Authenticate(r, s)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if gotID != "u-1" {
		t.Errorf("userID = %q, want u-1", gotID)
	}
}

// TestAuthenticate_QueryTokenFallback covers the spec alternative:
// when the Authorization header is missing the handler must accept
// ?token=... on the query string.
func TestAuthenticate_QueryTokenFallback(t *testing.T) {
	s := newTestSigner(t)
	tok := issueTestToken(t, s, &contracts.User{ID: "u-2", Username: "bob", Role: contracts.RoleDeveloper})

	r := httptest.NewRequest(http.MethodGet, "/ws?token="+url.QueryEscape(tok), nil)
	gotID, err := Authenticate(r, s)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if gotID != "u-2" {
		t.Errorf("userID = %q, want u-2", gotID)
	}
}

// TestAuthenticate_MissingToken returns a 401-class APIError when no
// credential is supplied at all.
func TestAuthenticate_MissingToken(t *testing.T) {
	s := newTestSigner(t)
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	_, err := Authenticate(r, s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("expected *contracts.APIError, got %T", err)
	}
	if apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("Code = %v, want %v", apiErr.Code, contracts.CodeUnauthorized)
	}
	if apiErr.Code.HTTPStatus() != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want 401", apiErr.Code.HTTPStatus())
	}
}

// TestAuthenticate_InvalidToken returns a 401 APIError when the
// supplied token is garbage.
func TestAuthenticate_InvalidToken(t *testing.T) {
	s := newTestSigner(t)
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer not-a-jwt")
	_, err := Authenticate(r, s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("expected *contracts.APIError, got %T", err)
	}
	if apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("Code = %v, want %v", apiErr.Code, contracts.CodeUnauthorized)
	}
}

// TestAuthenticate_BadScheme rejects anything that isn't Bearer.
func TestAuthenticate_BadScheme(t *testing.T) {
	s := newTestSigner(t)
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	_, err := Authenticate(r, s)
	if err == nil {
		t.Fatal("expected error for Basic auth, got nil")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("expected UNAUTHORIZED APIError, got %v", err)
	}
}

// TestAuthenticate_WrongSecret returns 401 when a token is signed
// with a different secret than the verifier expects.
func TestAuthenticate_WrongSecret(t *testing.T) {
	other, err := auth.NewSigner("a-different-secret", time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	tok := issueTestToken(t, other, &contracts.User{ID: "u-9", Role: contracts.RoleOperator})

	s := newTestSigner(t)
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	_, err = Authenticate(r, s)
	if err == nil {
		t.Fatal("expected error for cross-secret token, got nil")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("expected UNAUTHORIZED APIError, got %v", err)
	}
}

// TestAuthenticate_ExpiredToken returns 401 on an expired token.
// We mint a token by hand so the exp claim is unambiguously in the
// past, regardless of how NewSigner normalises ttl.
func TestAuthenticate_ExpiredToken(t *testing.T) {
	s := newTestSigner(t)

	// Build a JWT with exp=1 (1970-01-01T00:00:01Z, long past).
	expired := mintRawJWT(t, "test-secret-do-not-reuse", "u-exp", "alice", contracts.RoleOperator, 1)

	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Set("Authorization", "Bearer "+expired)
	_, err := Authenticate(r, s)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("expected UNAUTHORIZED APIError, got %v", err)
	}
}
