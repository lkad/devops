package rbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// userContextKey is the Gin context key the auth middleware uses
// to stash the authenticated *contracts.User. The RBAC middleware
// reads it back to evaluate permissions.
const userContextKey = "auth.user"

// withUser is a test helper that builds a Gin context pre-populated
// with the supplied user. It mirrors the real auth middleware so
// the RBAC tests do not have to re-implement JWT validation.
func withUser(u *contracts.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		if u != nil {
			c.Set(userContextKey, u)
		}
		c.Next()
	}
}

// TestMiddleware_RequirePermission_Allows is the spec scenario
// "Operator has deploy and config permissions" and the chain
// story: an Operator hits a protected endpoint and the request
// reaches the handler.
func TestMiddleware_RequirePermission_Allows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	r := gin.New()
	r.GET("/devices",
		withUser(&contracts.User{ID: "u", Role: contracts.RoleOperator}),
		RequirePermission(svc, PermissionViewDevices),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestMiddleware_RequirePermission_DeniesWithEnvelope is the spec
// scenario "Auditor has read-only access" / "Operator restricted
// on production restart": the response must be a 403 carrying
// the standard ErrorResponse envelope so clients can branch on
// the machine-readable code.
func TestMiddleware_RequirePermission_DeniesWithEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	r := gin.New()
	r.GET("/devices/:id/config",
		withUser(&contracts.User{ID: "u", Role: contracts.RoleAuditor}),
		RequirePermission(svc, PermissionModifyConfig),
		func(c *gin.Context) {
			t.Fatal("handler should not run on 403")
		},
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/devices/42/config", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var got contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("envelope not JSON: %v", err)
	}
	if got.Error.Code != contracts.CodeForbidden {
		t.Errorf("expected FORBIDDEN, got %q", got.Error.Code)
	}
}

// TestMiddleware_RequirePermission_NoUser is the spec scenario
// "Request with valid token but insufficient permissions" pushed
// to its boundary: a request that somehow has no authenticated
// user attached must 403, not 500 or panic. The auth middleware
// is responsible for the 401 path; the RBAC middleware is the
// last line of defense and fails closed.
func TestMiddleware_RequirePermission_NoUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	r := gin.New()
	r.GET("/devices",
		// intentionally no withUser — no principal in context
		RequirePermission(svc, PermissionViewDevices),
		func(c *gin.Context) {
			t.Fatal("handler should not run when no user")
		},
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var got contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("envelope not JSON: %v", err)
	}
	if got.Error.Code != contracts.CodeForbidden {
		t.Errorf("expected FORBIDDEN, got %q", got.Error.Code)
	}
}

// TestMiddleware_FullChain exercises the documented middleware
// order (Auth → RBAC → Handler) and confirms the chain produces
// a 403 envelope on insufficient privilege and a 200 on success.
func TestMiddleware_FullChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService()

	cases := []struct {
		name       string
		user       *contracts.User
		wantStatus int
	}{
		{"superadmin allowed", &contracts.User{ID: "u1", Role: contracts.RoleSuperAdmin}, http.StatusOK},
		{"operator allowed", &contracts.User{ID: "u2", Role: contracts.RoleOperator}, http.StatusOK},
		{"auditor denied", &contracts.User{ID: "u3", Role: contracts.RoleAuditor}, http.StatusForbidden},
		{"developer denied", &contracts.User{ID: "u4", Role: contracts.RoleDeveloper}, http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := gin.New()
			r.PUT("/devices/:id/config",
				withUser(c.user),
				RequirePermission(svc, PermissionModifyConfig),
				func(gc *gin.Context) {
					gc.JSON(http.StatusOK, gin.H{"ok": true})
				},
			)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/devices/42/config", nil)
			r.ServeHTTP(w, req)
			if w.Code != c.wantStatus {
				t.Errorf("role=%s: got %d want %d body=%s", c.user.Role, w.Code, c.wantStatus, w.Body.String())
			}
			if c.wantStatus == http.StatusForbidden {
				var env contracts.ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
					t.Fatalf("expected error envelope: %v", err)
				}
				if env.Error.Code != contracts.CodeForbidden {
					t.Errorf("expected FORBIDDEN, got %q", env.Error.Code)
				}
			}
		})
	}
}

// TestMiddleware_403CarriesCodeForbidden is a guard against
// regression: the spec dictates the FORBIDDEN code; an internal
// refactor that changes the code would break client switches.
func TestMiddleware_403CarriesCodeForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	r := gin.New()
	r.POST("/devices/:id/restart",
		withUser(&contracts.User{ID: "u", Role: contracts.RoleAuditor}),
		RequirePermission(svc, PermissionRemoteRestart),
		func(c *gin.Context) {},
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/devices/42/restart", nil)
	r.ServeHTTP(w, req)

	var env contracts.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON envelope: %v", err)
	}
	if env.Error.Code != contracts.CodeForbidden {
		t.Errorf("error.code = %q, want FORBIDDEN", env.Error.Code)
	}
}
