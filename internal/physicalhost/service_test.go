package physicalhost

import (
	"context"
	"errors"
	"testing"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestService_GetHost_CrossTenant_Denied covers the v0.3.0.0
// P0 #2 cross-tenant enforcement: a non-SuperAdmin caller
// without membership in any of the host's project links is
// denied access.
func TestService_GetHost_CrossTenant_Denied(t *testing.T) {
	repo := NewRepository(openDB(t))
	svc := NewService(ServiceConfig{
		Repo: repo,
		ProjectIDsForHost: func(hostID string) ([]string, error) {
			return []string{"proj-x"}, nil
		},
	})
	cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleDeveloper})
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.GetWithCaller(ctx, "any-id")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// TestService_GetHost_SuperAdmin_Bypasses covers the spec
// rule that SuperAdmin is implicitly allowed to read any
// host.
func TestService_GetHost_SuperAdmin_Bypasses(t *testing.T) {
	repo := NewRepository(openDB(t))
	svc := NewService(ServiceConfig{Repo: repo})
	cl := caller.New(&contracts.User{ID: "root", Username: "root", Role: contracts.RoleSuperAdmin})
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.GetWithCaller(ctx, "any-id")
	// The repo returns not-found (the fixture has no rows)
	// but the cross-tenant guard MUST NOT short-circuit
	// with ErrForbidden.
	if errors.Is(err, ErrForbidden) {
		t.Errorf("SuperAdmin should bypass cross-tenant, got ErrForbidden")
	}
}

// TestService_GetHost_NilCaller_401 covers the fail-closed
// rule: a context without a caller MUST surface as
// ErrUnauthenticated.
func TestService_GetHost_NilCaller_401(t *testing.T) {
	repo := NewRepository(openDB(t))
	svc := NewService(ServiceConfig{Repo: repo})
	_, err := svc.GetWithCaller(context.Background(), "any-id")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}
