package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/auth/ldap"
	"github.com/devops-toolkit/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

type Handler struct {
	ldapClient *ldap.Client
	config     *config.AuthConfig
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      User   `json:"user"`
}

type User struct {
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type Claims struct {
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

func NewHandler(ldapClient *ldap.Client, cfg *config.AuthConfig) *Handler {
	return &Handler{
		ldapClient: ldapClient,
		config:     cfg,
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apierror.MethodNotAllowed(w)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.ValidationError(w, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		apierror.ValidationError(w, "username and password required")
		return
	}

	var roles []string
	var userDN string

	// Dev bypass mode: use hardcoded credentials
	if h.config.DevBypass && h.ldapClient == nil {
		if req.Username != h.config.DevUsername || req.Password != h.config.DevPassword {
			apierror.Unauthorized(w, "invalid credentials")
			return
		}
		roles = h.config.DevRoles
		if roles == nil {
			roles = []string{"Developer"}
		}
		userDN = "uid=" + req.Username + ",ou=Users,dc=example,dc=com"
	} else {
		// Normal LDAP authentication
		if h.ldapClient == nil {
			apierror.ServiceUnavailable(w, "authentication unavailable")
			return
		}

		// Search for user DN in LDAP
		userDN = "uid=" + req.Username + ",ou=Users,dc=example,dc=com"

		// Authenticate against LDAP
		ok, err := h.ldapClient.Authenticate(userDN, req.Password)
		if err != nil {
			apierror.InternalError(w, "authentication error")
			return
		}
		if !ok {
			apierror.Unauthorized(w, "invalid credentials")
			return
		}

		// Get roles from LDAP
		roles, err = h.ldapClient.GetRoles(userDN)
		if err != nil {
			apierror.InternalError(w, "failed to get roles")
			return
		}
	}

	// Default permissions based on roles
	permissions := getPermissionsForRoles(roles)

	// Parse session duration
	duration := 8 * time.Hour
	if h.config.SessionDuration != "" {
		if d, err := time.ParseDuration(h.config.SessionDuration); err == nil {
			duration = d
		}
	}

	// Create JWT
	expiresAt := time.Now().Add(duration)
	claims := Claims{
		Username:    req.Username,
		Roles:       roles,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userDN,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.config.JWTSecret))
	if err != nil {
		apierror.InternalError(w, "failed to create token")
		return
	}

	json.NewEncoder(w).Encode(LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User: User{
			Username:    req.Username,
			Roles:       roles,
			Permissions: permissions,
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Logout is client-side: we just tell client to discard the token
	// In a production system, you'd maintain a token blacklist
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apierror.MethodNotAllowed(w)
		return
	}

	// Get user from context (set by middleware)
	user, ok := r.Context().Value(userContextKey).(*User)
	if !ok || user == nil {
		apierror.Unauthorized(w, "not authenticated")
		return
	}

	json.NewEncoder(w).Encode(user)
}

func getPermissionsForRoles(roles []string) []string {
	permSet := make(map[string]bool)
	for _, role := range roles {
		switch role {
		case "SuperAdmin":
			permSet["all"] = true
		case "Operator":
			permSet["deploy"] = true
			permSet["config-manage"] = true
			permSet["execute"] = true
			permSet["read"] = true
		case "Developer":
			permSet["read"] = true
			permSet["test-deploy"] = true
		case "Auditor":
			permSet["read"] = true
			permSet["audit-read"] = true
		}
	}

	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}
	return perms
}
