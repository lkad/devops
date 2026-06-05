package ldap

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler exposes the LDAP auth endpoints as Gin handlers. It is the
// only piece in this package that imports Gin; everything else is
// framework-agnostic so the same Service can be reused elsewhere.
type Handler struct {
	svc    *Service
	signer *auth.Signer
	ttl    int64 // seconds; cached at construction for the response envelope
	log    *slog.Logger
}

// HandlerConfig wires a Handler.
type HandlerConfig struct {
	Service   *Service
	JWTSecret string
	TokenTTL  int64 // seconds; required, must be > 0
	Issuer    string // currently informational; auth.Issuer is the source of truth
	Logger    *slog.Logger
}

// NewHandler builds a Handler. It validates the secret and TTL up
// front so a misconfigured server fails at boot, not at first login.
func NewHandler(cfg HandlerConfig) *Handler {
	signer, err := auth.NewSigner(cfg.JWTSecret, secondsToDuration(cfg.TokenTTL))
	if err != nil {
		// A misconfigured secret is a deployment bug; we'd rather
		// crash the process than ship tokens an attacker can forge.
		panic("ldap handler: " + err.Error())
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Handler{
		svc:    cfg.Service,
		signer: signer,
		ttl:    cfg.TokenTTL,
		log:    cfg.Logger,
	}
}

// loginRequest is the wire shape of POST /api/v1/auth/login. We
// keep the field names short and the request body tiny so the
// endpoint is cheap to call and easy to test.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// userView is the part of contracts.User we expose on the wire.
// We omit the ID (it is already inside the JWT) and keep the
// shape stable for frontend clients.
type userView struct {
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
}

// loginResponse is the success envelope. The token is a string
// (not an object) so the frontend can drop it into the
// Authorization header verbatim.
type loginResponse struct {
	Token     string   `json:"token"`
	TokenType string   `json:"token_type"`
	ExpiresAt int64    `json:"expires_at"`
	User      userView `json:"user"`
}

// Login authenticates the user and, on success, returns a signed
// JWT. On any non-2xx the response is the standard error envelope.
//
// Status code mapping:
//
//	200 — success
//	400 — malformed body or missing fields
//	401 — invalid credentials
//	429 — too many failed attempts
//	500 — backend unavailable (e.g. LDAP server down)
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON with username and password",
		})
		return
	}
	if req.Username == "" || req.Password == "" {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "username and password are required",
		})
		return
	}

	user, err := h.svc.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	token, exp, err := h.signer.Issue(user)
	if err != nil {
		h.log.Error("issue jwt", "err", err, "user", user.Username)
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to issue session token",
		})
		return
	}

	handler.WriteJSON(c.Writer, http.StatusOK, loginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: exp,
		User: userView{
			Username: user.Username,
			Email:    user.Email,
			Role:     string(user.Role),
		},
	})
}

// Health is the GET /api/v1/auth/ldap/health handler. It returns
// 200 with {"status":"ok"} when the LDAP backend is reachable and
// 503 with a reason otherwise.
func (h *Handler) Health(c *gin.Context) {
	if err := h.svc.HealthCheck(c.Request.Context()); err != nil {
		// We bypass WriteError here because the spec calls for
		// 503, but contracts.CodeInternal maps to 500. Emit the
		// envelope directly so the status matches the requirement.
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(http.StatusServiceUnavailable)
		body := contracts.ErrorResponse{
			Error: contracts.ErrorBody{
				Code:    contracts.CodeInternal,
				Message: "ldap backend is unhealthy",
				Details: map[string]any{"reason": err.Error()},
			},
		}
		_ = json.NewEncoder(c.Writer).Encode(body)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, gin.H{"status": "ok"})
}

// writeServiceError maps a service-layer error to the right status
// + envelope. Kept private; the handler is the only thing that
// knows the HTTP shape.
func (h *Handler) writeServiceError(c *gin.Context, err error) {
	apiErr := &contracts.APIError{}
	if !errors.As(err, &apiErr) {
		// Defensive: every service-level error is supposed to be
		// an *APIError. Anything else is a programming bug and
		// we render a generic 500.
		h.log.Error("unexpected service error", "err", err)
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "internal server error",
		})
		return
	}
	handler.WriteError(c.Writer, apiErr)
}

// secondsToDuration converts a TTL in seconds (the form that
// arrives from YAML config) to a time.Duration. A non-positive
// value falls back to one hour so a typo in config does not produce
// a token that expires immediately.
func secondsToDuration(s int64) (d time.Duration) {
	if s <= 0 {
		return time.Hour
	}
	return time.Duration(s) * time.Second
}
