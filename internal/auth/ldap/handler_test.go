package ldap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestHandler_Login_Success covers "Valid LDAP credentials" end-to-end:
// a POST to /api/v1/auth/login with a known dev user returns 200 and
// a JWT in the response body.
func TestHandler_Login_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret-do-not-use", TokenTTL: 600, Issuer: "devops-toolkit"})

	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "alice-pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Errorf("expected non-empty token")
	}
	if resp.User.Username != "alice" {
		t.Errorf("expected username alice, got %q", resp.User.Username)
	}
	if resp.User.Role != string(contracts.RoleOperator) {
		t.Errorf("expected Operator role, got %q", resp.User.Role)
	}
	if resp.ExpiresAt == 0 {
		t.Errorf("expected non-zero ExpiresAt")
	}
}

// TestHandler_Login_InvalidCredentials covers "Invalid LDAP credentials":
// a bad password returns 401 with a UNAUTHORIZED code in the envelope.
func TestHandler_Login_InvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret", TokenTTL: 600})

	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "WRONG"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "UNAUTHORIZED") {
		t.Errorf("expected UNAUTHORIZED in body, got %s", rr.Body.String())
	}
}

// TestHandler_Login_RateLimit covers the per-username lockout
// observable at the HTTP layer: after N failed logins, the next call
// returns 429 RATE_LIMITED.
func TestHandler_Login_RateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newServiceForTest(newDevFake(), 3, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret", TokenTTL: 600})

	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	// Burn the budget with bad passwords.
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(loginRequest{Username: "alice", Password: "WRONG"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
	}
	// 4th attempt — even with the correct password — must be 429.
	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "alice-pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "RATE_LIMITED") {
		t.Errorf("expected RATE_LIMITED in body, got %s", rr.Body.String())
	}
}

// TestHandler_Login_BadJSON exercises the "VALIDATION_ERROR" branch.
// A malformed body must not panic and must return 400.
func TestHandler_Login_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret", TokenTTL: 600})

	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("expected VALIDATION_ERROR in body, got %s", rr.Body.String())
	}
}

// TestHandler_Login_MissingFields returns 400 when username or password
// is empty. Empty fields are a validation failure, not an auth failure.
func TestHandler_Login_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret", TokenTTL: 600})

	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)

	for _, body := range []string{`{"username":"","password":""}`, `{"username":"x"}`, `{"password":"x"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d", body, rr.Code)
		}
	}
}

// TestHandler_HealthCheck covers the spec's "Health Check" scenarios.
// It exposes a Gin route that delegates to the service.
func TestHandler_HealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newDevFake()
	f.SetHealthy(true)
	svc := newServiceForTest(f, 100, time.Minute)
	h := NewHandler(HandlerConfig{Service: svc, JWTSecret: "test-secret", TokenTTL: 600})

	r := gin.New()
	r.GET("/api/v1/auth/ldap/health", h.Health)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/ldap/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for healthy, got %d", rr.Code)
	}

	f.SetHealthy(false)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/ldap/health", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for unhealthy, got %d", rr.Code)
	}
}

// Compile-time guard: NewHandler must not panic on a sane config.
var _ = func() *Handler {
	return NewHandler(HandlerConfig{
		Service:   newServiceForTest(NewFake(), 1, time.Second),
		JWTSecret: "x",
		TokenTTL: 60,
		Issuer:    "test",
	})
}()

// silence the unused import warning when only some tests are run.
var _ = context.Background
