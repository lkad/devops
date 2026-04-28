package auth

import (
	"net/http"
	"time"

	"github.com/devops-toolkit/internal/auth/ldap"
	"github.com/devops-toolkit/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Handler struct {
	ldapClient *ldap.Client
	config     *config.AuthConfig
}

func NewHandler(ldapClient *ldap.Client, cfg *config.AuthConfig) *Handler {
	return &Handler{
		ldapClient: ldapClient,
		config:     cfg,
	}
}

func (h *Handler) LoginGin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	var roles []string
	var userDN string

	// Dev bypass mode: use hardcoded credentials
	if h.config.DevBypass && h.ldapClient == nil {
		if req.Username != h.config.DevUsername || req.Password != h.config.DevPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
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
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication unavailable"})
			return
		}

		// Search for user DN in LDAP
		userDN = "uid=" + req.Username + ",ou=Users,dc=example,dc=com"

		// Authenticate against LDAP
		ok, err := h.ldapClient.Authenticate(userDN, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication error"})
			return
		}
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		// Get roles from LDAP
		roles, err = h.ldapClient.GetRoles(userDN)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get roles"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      tokenString,
		"expiresAt":  expiresAt.Unix(),
		"user": User{
			Username:    req.Username,
			Roles:       roles,
			Permissions: permissions,
		},
	})
}

func (h *Handler) LogoutGin(c *gin.Context) {
	// Logout is client-side: we just tell client to discard the token
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) MeGin(c *gin.Context) {
	// Get user from context (set by middleware)
	user, ok := c.Get("user")
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	c.JSON(http.StatusOK, user)
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
