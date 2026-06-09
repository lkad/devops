package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/ldap"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestAuthFlow_LoginThenProtected covers the full production
// auth path: login through the LDAP service, receive a JWT,
// hit a protected endpoint with the JWT, expect 200. This is
// the integration contract for "LDAP works end-to-end" without
// standing up a real LDAP server — the Fake client is what the
// dev config uses.
func TestAuthFlow_LoginThenProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := auth.NewSigner("test-flow-secret", time.Hour)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	// Wire a fresh Fake LDAP client preloaded with the four
	// canonical dev users. Group DNs must match the spec's
	// defaultGroupRoleMap (FQDN form).
	fakeLDAP := ldap.NewFake()
	fakeLDAP.AddUser("alice", "alice123", "alice@example.com", []string{"cn=SRE_Lead,ou=Groups,dc=example,dc=com"})
	fakeLDAP.AddUser("bob", "bob123", "bob@example.com", []string{"cn=IT_Ops,ou=Groups,dc=example,dc=com"})

	svc := ldap.NewService(ldap.ServiceConfig{
		Client:          fakeLDAP,
		MaxFailedLogins: 5,
		RateLimitWindow: time.Minute,
	})
	loginH := ldap.NewHandler(ldap.HandlerConfig{Service: svc, JWTSecret: "test-flow-secret", TokenTTL: 3600, Logger: nil})

	// Stand up two routers:
	//   1) the login router (no auth) — POST /login
	//   2) the protected router — GET /protected with the
	//      NewAuthMiddleware enforcing PermissionViewPhysicalHosts
	r := gin.New()
	r.POST("/login", loginH.Login)
	r.GET("/protected", auth.NewAuthMiddleware(auth.AuthMiddlewareConfig{
		Signer:        signer,
		PermissionSvc: rbac.NewService(),
		RequiredPerm:  rbac.PermissionViewPhysicalHosts,
	}), func(c *gin.Context) {
		v, _ := c.Get(rbac.AuthUserKey)
		u, _ := v.(*contracts.User)
		c.JSON(http.StatusOK, gin.H{"user": u.ID, "role": u.Role})
	})

	// 1. Login with bad password → 401.
	badReq := httptest.NewRequest("POST", "/login", bytes.NewBufferString(`{"username":"alice","password":"wrong"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badRR := httptest.NewRecorder()
	r.ServeHTTP(badRR, badReq)
	if badRR.Code != 401 {
		t.Errorf("bad password: status = %d, want 401; body = %s", badRR.Code, badRR.Body.String())
	}

	// 2. Login with good password → 200 + JWT in the response.
	goodReq := httptest.NewRequest("POST", "/login", bytes.NewBufferString(`{"username":"alice","password":"alice123"}`))
	goodReq.Header.Set("Content-Type", "application/json")
	goodRR := httptest.NewRecorder()
	r.ServeHTTP(goodRR, goodReq)
	if goodRR.Code != http.StatusOK {
		t.Fatalf("good login: status = %d, body = %s", goodRR.Code, goodRR.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(goodRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("login response missing token")
	}
	if resp.User.Username != "alice" {
		t.Errorf("username = %q, want alice", resp.User.Username)
	}

	// 3. Hit the protected endpoint with the JWT.
	protReq := httptest.NewRequest("GET", "/protected", nil)
	protReq.Header.Set("Authorization", "Bearer "+resp.Token)
	protRR := httptest.NewRecorder()
	r.ServeHTTP(protRR, protReq)
	if protRR.Code != http.StatusOK {
		t.Errorf("protected with JWT: status = %d, body = %s", protRR.Code, protRR.Body.String())
	}
}

// TestAuthFlow_DevBypassStillWorks: the dev config's X-User
// bypass must still let the protected route through so the
// frontend dev server can run without LDAP.
func TestAuthFlow_DevBypassStillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", auth.NewAuthMiddleware(auth.AuthMiddlewareConfig{
		DevBypass:    true,
		PermissionSvc: rbac.NewService(),
		RequiredPerm:  rbac.PermissionViewPhysicalHosts,
	}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-User", "alice")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("dev bypass: status = %d, want 200", rr.Code)
	}
}
