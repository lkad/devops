package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	s, err := NewSigner("test-middleware-secret", time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func newRouter(t *testing.T, cfg AuthMiddlewareConfig) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", NewAuthMiddleware(cfg), func(c *gin.Context) {
		v, _ := c.Get(rbac.AuthUserKey)
		u, _ := v.(*contracts.User)
		c.JSON(http.StatusOK, gin.H{"id": u.ID, "role": u.Role})
	})
	return r
}

func TestAuthMiddleware_DevBypass(t *testing.T) {
	r := newRouter(t, AuthMiddlewareConfig{
		DevBypass: true,
		PermissionSvc: rbac.NewService(),
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-User", "alice")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	r := newRouter(t, AuthMiddlewareConfig{
		Signer: newTestSigner(t),
		PermissionSvc: rbac.NewService(),
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	s := newTestSigner(t)
	tok, _, err := s.Issue(&contracts.User{ID: "u-1", Username: "alice", Role: contracts.RoleOperator})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	r := newRouter(t, AuthMiddlewareConfig{
		Signer: s,
		PermissionSvc: rbac.NewService(),
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_RequiresPermission(t *testing.T) {
	s := newTestSigner(t)
	tok, _, _ := s.Issue(&contracts.User{ID: "u-2", Username: "bob", Role: contracts.RoleDeveloper})
	r := newRouter(t, AuthMiddlewareConfig{
		Signer: s,
		PermissionSvc: rbac.NewService(),
		RequiredPerm: rbac.PermissionMaintenancePhysical,
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Errorf("status = %d, want 403 (Developer denied maintenance)", rr.Code)
	}
}
