package caller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestFromContext_AbsentReturnsFalse covers the "auth context
// is empty" path: no caller on the context means the caller
// cannot be retrieved. The helper returns (nil, false) so
// callers can branch on ok without nil-checking the user.
func TestFromContext_AbsentReturnsFalse(t *testing.T) {
	cl, ok := FromContext(context.Background())
	if ok || cl != nil {
		t.Fatalf("expected (nil, false); got (%v, %v)", cl, ok)
	}
}

// TestWithContext_ThenFromContext_RoundTrip pins the
// happy-path: attach a caller, retrieve it, check the
// fields survive the round trip.
func TestWithContext_ThenFromContext_RoundTrip(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleOperator}
	in := New(u)
	ctx := WithContext(context.Background(), in)
	out, ok := FromContext(ctx)
	if !ok || out == nil {
		t.Fatalf("expected caller on context; got (%v, %v)", out, ok)
	}
	if out.UserID() != "u-1" {
		t.Errorf("expected user id u-1; got %q", out.UserID())
	}
	if out.User.Role != contracts.RoleOperator {
		t.Errorf("expected role Operator; got %q", out.User.Role)
	}
}

// TestIsMemberOf_SuperAdminIsImplicit pins the SuperAdmin
// escape hatch: a SuperAdmin is a member of every project
// without a DB lookup. The MembershipChecker is nil to
// prove the check short-circuits BEFORE the lookup.
func TestIsMemberOf_SuperAdminIsImplicit(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleSuperAdmin}
	cl := New(u)
	if !cl.IsMemberOf(context.Background(), "any-project", nil) {
		t.Fatal("SuperAdmin must be a member of every project")
	}
}

// TestIsMemberOf_NilCaller is the "missing auth context"
// case: a nil receiver is treated as "not a member" so a
// service-layer call that forgot to attach the auth
// context does not accidentally grant access.
func TestIsMemberOf_NilCaller(t *testing.T) {
	var c *Caller
	if c.IsMemberOf(context.Background(), "p1", nil) {
		t.Fatal("nil caller must not be a member")
	}
}

// TestIsMemberOf_EmptyProjectID refuses an uninitialised
// variable. A service method that forgot to populate
// projectID must not accidentally pass the check.
func TestIsMemberOf_EmptyProjectID(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleOperator}
	cl := New(u)
	checker := func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return map[string]struct{}{"p1": {}}, nil
	}
	if cl.IsMemberOf(context.Background(), "", checker) {
		t.Fatal("empty projectID must deny access")
	}
}

// TestIsMemberOf_NilChecker is the "no memberships" default:
// a nil MembershipChecker is a fail-closed default that
// prevents a misconfigured service from accidentally
// granting every caller access to every project.
func TestIsMemberOf_NilChecker(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleOperator}
	cl := New(u)
	if cl.IsMemberOf(context.Background(), "p1", nil) {
		t.Fatal("nil checker must deny access for non-SuperAdmin")
	}
}

// TestIsMemberOf_MembershipCache populates the cache on
// the first lookup and re-uses it on subsequent calls. The
// "called" counter proves the checker is invoked only once.
func TestIsMemberOf_MembershipCache(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleOperator}
	cl := New(u)
	calls := 0
	checker := func(ctx context.Context, userID string) (map[string]struct{}, error) {
		calls++
		return map[string]struct{}{"p1": {}, "p2": {}}, nil
	}
	if !cl.IsMemberOf(context.Background(), "p1", checker) {
		t.Fatal("expected member of p1")
	}
	if !cl.IsMemberOf(context.Background(), "p2", checker) {
		t.Fatal("expected member of p2")
	}
	if cl.IsMemberOf(context.Background(), "p3", checker) {
		t.Fatal("expected NOT a member of p3")
	}
	if calls != 1 {
		t.Errorf("checker should be called once; got %d", calls)
	}
}

// TestIsMemberOf_CheckerError is the "DB error" path: when
// the membership lookup fails the check is fail-closed
// (returns false). The cache is also poisoned with the
// error so subsequent calls don't re-trigger the broken
// DB query.
func TestIsMemberOf_CheckerError(t *testing.T) {
	u := &contracts.User{ID: "u-1", Role: contracts.RoleOperator}
	cl := New(u)
	checker := func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return nil, errors.New("db down")
	}
	if cl.IsMemberOf(context.Background(), "p1", checker) {
		t.Fatal("expected fail-closed on checker error")
	}
	// Subsequent calls must also fail closed (cached error).
	if cl.IsMemberOf(context.Background(), "p1", checker) {
		t.Fatal("expected fail-closed on cached error")
	}
}

// TestIsSuperAdmin covers the convenience method. A nil
// receiver returns false (not a panic).
func TestIsSuperAdmin(t *testing.T) {
	cases := []struct {
		name string
		u    *contracts.User
		want bool
	}{
		{"nil user", nil, false},
		{"operator", &contracts.User{Role: contracts.RoleOperator}, false},
		{"superadmin", &contracts.User{Role: contracts.RoleSuperAdmin}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var cl *Caller
			if c.u != nil {
				cl = New(c.u)
			}
			if got := cl.IsSuperAdmin(); got != c.want {
				t.Errorf("IsSuperAdmin() = %v, want %v", got, c.want)
			}
		})
	}
}

// stubChecker is a tiny MembershipChecker factory used by
// the RequireProjectAccess tests. The returned function
// reports the supplied set of project IDs and counts how
// many times it was invoked so a test can assert the
// per-request cache is honoured.
func stubChecker(projects map[string]struct{}) (MembershipChecker, *int) {
	calls := 0
	return func(_ context.Context, _ string) (map[string]struct{}, error) {
		calls++
		return projects, nil
	}, &calls
}

// alwaysAllow is a permission checker that always returns
// true; used when a test wants to isolate the membership
// half of the check.
func alwaysAllow(*contracts.User, string) bool { return true }

// alwaysDeny is a permission checker that always returns
// false; used when a test wants to verify the permission
// half of the check.
func alwaysDeny(*contracts.User, string) bool { return false }

// runMiddleware builds a Gin engine, attaches the supplied
// middleware to GET /, and returns the recorder so the
// test can assert the status / body. The caller parameter
// is the Caller stamped on the context; pass nil to skip
// the stamp entirely (the no-auth path).
func runMiddleware(t *testing.T, cl *Caller, mw gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x/:id", func(c *gin.Context) {
		if cl != nil {
			callerSet(c, cl)
		}
		mw(c)
		if !c.IsAborted() {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		}
	})
	req := httptest.NewRequest("GET", "/x/proj-A", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// callerSet is a thin shim so the test does not have to
// import the caller package's public API surface for
// every stamp.
func callerSet(c *gin.Context, cl *Caller) {
	WithGin(c, cl)
}

// TestRequireProjectAccess_DeniesDeveloperInOtherProject
// pins the P0 cross-tenant fix: a Developer who is a
// member of project A must get 403 when calling a
// project-B endpoint.
func TestRequireProjectAccess_DeniesDeveloperInOtherProject(t *testing.T) {
	cl := New(&contracts.User{ID: "dev-1", Role: contracts.RoleDeveloper})
	checker, _ := stubChecker(map[string]struct{}{"proj-A": {}})
	// The URL is /x/proj-B but the caller is only a member
	// of proj-A; the middleware must 403.
	mw := RequireProjectAccess(checker, alwaysAllow, func(c *gin.Context) string {
		return c.Param("id")
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x/:id", func(c *gin.Context) {
		WithGin(c, cl)
		mw(c)
		if !c.IsAborted() {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		}
	})
	req := httptest.NewRequest("GET", "/x/proj-B", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), "FORBIDDEN") {
		t.Errorf("expected FORBIDDEN code in body; got %s", w.Body.String())
	}
}

// TestRequireProjectAccess_AllowsDeveloperInSameProject
// is the happy-path counterpart: a Developer who IS a
// member of the project gets through when the permission
// checker also allows.
func TestRequireProjectAccess_AllowsDeveloperInSameProject(t *testing.T) {
	cl := New(&contracts.User{ID: "dev-1", Role: contracts.RoleDeveloper})
	checker, _ := stubChecker(map[string]struct{}{"proj-A": {}})
	mw := RequireProjectAccess(checker, alwaysAllow, func(c *gin.Context) string {
		return c.Param("id")
	})
	w := runMiddleware(t, cl, mw)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", w.Code, w.Body.String())
	}
}

// TestRequireProjectAccess_AllowsSuperAdmin is the
// escape-hatch pin: a SuperAdmin is implicitly a member
// of every project. The MembershipChecker is nil to prove
// the short-circuit runs BEFORE the lookup.
func TestRequireProjectAccess_AllowsSuperAdmin(t *testing.T) {
	cl := New(&contracts.User{ID: "root", Role: contracts.RoleSuperAdmin})
	mw := RequireProjectAccess(nil, alwaysAllow, func(c *gin.Context) string {
		return c.Param("id")
	})
	w := runMiddleware(t, cl, mw)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", w.Code, w.Body.String())
	}
}

// TestRequireProjectAccess_DeniesWhenPermCheckerDenies
// pins the second half of the check: a caller who is a
// member of the project can still be denied when the
// per-project permission checker returns false (e.g. a
// Viewer trying to write).
func TestRequireProjectAccess_DeniesWhenPermCheckerDenies(t *testing.T) {
	cl := New(&contracts.User{ID: "viewer-1", Role: contracts.RoleOperator})
	checker, _ := stubChecker(map[string]struct{}{"proj-A": {}})
	mw := RequireProjectAccess(checker, alwaysDeny, func(c *gin.Context) string {
		return c.Param("id")
	})
	w := runMiddleware(t, cl, mw)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", w.Code, w.Body.String())
	}
}

// TestRequireProjectAccess_DeniesWhenNoCaller is the
// "auth middleware never ran" path: the gin context has
// no Caller stashed, so the middleware must 403 rather
// than panic or pass through.
func TestRequireProjectAccess_DeniesWhenNoCaller(t *testing.T) {
	mw := RequireProjectAccess(nil, alwaysAllow, func(c *gin.Context) string {
		return c.Param("id")
	})
	w := runMiddleware(t, nil, mw)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", w.Code, w.Body.String())
	}
}

// TestRequireProjectAccess_DeniesEmptyProjectID is the
// "uninitialised variable" guard: a route that forgot to
// wire a projectIDFn returns "" and the middleware must
// 403.
func TestRequireProjectAccess_DeniesEmptyProjectID(t *testing.T) {
	cl := New(&contracts.User{ID: "dev-1", Role: contracts.RoleDeveloper})
	mw := RequireProjectAccess(nil, alwaysAllow, func(c *gin.Context) string {
		return ""
	})
	w := runMiddleware(t, cl, mw)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", w.Code, w.Body.String())
	}
}

// TestRequireProjectAccess_PermissionEnvelope pins the
// response shape: the 403 must use the standard
// contracts.ErrorResponse envelope so the frontend can
// parse it without a special case.
func TestRequireProjectAccess_PermissionEnvelope(t *testing.T) {
	cl := New(&contracts.User{ID: "dev-1", Role: contracts.RoleDeveloper})
	checker, _ := stubChecker(map[string]struct{}{"proj-A": {}})
	mw := RequireProjectAccess(checker, alwaysAllow, func(c *gin.Context) string {
		return c.Param("id")
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x/:id", func(c *gin.Context) {
		WithGin(c, cl)
		mw(c)
		if !c.IsAborted() {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		}
	})
	req := httptest.NewRequest("GET", "/x/proj-B", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type=%q want application/json", ct)
	}
	body := w.Body.String()
	if !contains(body, `"error"`) || !contains(body, "FORBIDDEN") {
		t.Errorf("body=%s; want error envelope with FORBIDDEN code", body)
	}
}

// TestCheckProjectAccess_BooleanFlavour covers the
// service-layer entry point: a background worker that
// holds a plain context.Context (no gin) can call
// CheckProjectAccess and get the same answer the
// middleware would produce.
func TestCheckProjectAccess_BooleanFlavour(t *testing.T) {
	cl := New(&contracts.User{ID: "dev-1", Role: contracts.RoleDeveloper})
	checker, _ := stubChecker(map[string]struct{}{"proj-A": {}})
	ctx := WithContext(context.Background(), cl)
	if !CheckProjectAccess(ctx, "proj-A", checker, alwaysAllow) {
		t.Fatal("expected allow for member of proj-A")
	}
	if CheckProjectAccess(ctx, "proj-B", checker, alwaysAllow) {
		t.Fatal("expected deny for non-member of proj-B")
	}
	// SuperAdmin shortcut.
	sa := New(&contracts.User{ID: "root", Role: contracts.RoleSuperAdmin})
	saCtx := WithContext(context.Background(), sa)
	if !CheckProjectAccess(saCtx, "proj-B", nil, nil) {
		t.Fatal("expected SuperAdmin to be allowed without a checker")
	}
}

// contains is a tiny substring helper so the test does
// not pull strings.Contains into the file's import list
// for a handful of calls.
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// Unused to keep imports tidy when the test file is
// short on references for some configurations.
var _ = errors.New
