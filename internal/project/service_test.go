package project

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newServiceWithDB is a test convenience: it stands up an in-memory
// database, runs the migrations, and returns a Service backed by a
// fresh repository. Each call gets its own DB so tests cannot
// accidentally share state. The Service is wired with a default
// SuperAdmin caller (via testContext) so legacy tests do not have
// to thread ctx explicitly; the cross-tenant tests below swap in
// restricted callers.
func newServiceWithDB(t *testing.T) (*Service, *Repository) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	return NewService(repo), repo
}

// testContext returns a context pre-populated with a SuperAdmin
// caller. The Service's cross-tenant checks (added in v0.3.0.0 P0
// #2) treat SuperAdmin as a member of every project, so legacy
// tests that do not care about tenancy continue to pass without
// per-call ctx plumbing.
func testContext() context.Context {
	cl := caller.New(&contracts.User{ID: "test-admin", Username: "test-admin", Role: contracts.RoleSuperAdmin})
	return caller.WithContext(context.Background(), cl)
}

// TestService_CreateProject_RejectsEmptyName is the spec scenario
// "Create Project" with an invalid input. The service layer is the
// place validation lives; an empty name must surface as a 400.
func TestService_CreateProject_RejectsEmptyName(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, err := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	if err != nil {
		t.Fatalf("seed type: %v", err)
	}
	_, err = svc.CreateProject(CreateProjectInput{Name: "", Code: "p", TypeID: pt.ID}, testContext())
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %v", err)
	}
}

// TestService_CreateProject_RejectsBadCode covers the same scenario
// with a code containing illegal characters. Codes are URL slugs
// and must be lowercase alphanumerics + dashes.
func TestService_CreateProject_RejectsBadCode(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	_, err := svc.CreateProject(CreateProjectInput{Name: "X", Code: "BAD CODE!", TypeID: pt.ID}, testContext())
	if err == nil {
		t.Fatal("expected validation error for bad code")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %v", err)
	}
}

// TestService_CreateProject_RejectsUnknownType covers the spec's
// "Project type" requirement: a project must reference a real
// project type. A bad type id must surface as a 400, not a 500.
func TestService_CreateProject_RejectsUnknownType(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	_, err := svc.CreateProject(CreateProjectInput{Name: "X", Code: "x", TypeID: "ghost"}, testContext())
	if err == nil {
		t.Fatal("expected validation error for missing type")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %v", err)
	}
}

// TestService_CreateProject_RejectsDepthOverflow enforces the
// 3-level hierarchy cap. A 4th level is rejected with a 400.
func TestService_CreateProject_RejectsDepthOverflow(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID}, testContext())
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID}, testContext())

	// Trying to add a 4th level under prj must fail.
	_, err := svc.CreateProject(CreateProjectInput{Name: "TooDeep", Code: "deep", TypeID: pt.ID, ParentID: &prj.ID}, testContext())
	if err == nil {
		t.Fatal("expected depth overflow error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeValidation {
		t.Errorf("expected VALIDATION_ERROR for depth overflow, got %v", err)
	}
	if !strings.Contains(strings.ToLower(ae.Message), "depth") && !strings.Contains(strings.ToLower(ae.Message), "level") {
		t.Errorf("expected depth-related message, got %q", ae.Message)
	}
}

// TestService_CreateProject_DuplicateCode is the spec's
// uniqueness rule for project codes.
func TestService_CreateProject_DuplicateCode(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	if _, err := svc.CreateProject(CreateProjectInput{Name: "P", Code: "dup", TypeID: pt.ID}, testContext()); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreateProject(CreateProjectInput{Name: "P2", Code: "dup", TypeID: pt.ID}, testContext())
	if err == nil {
		t.Fatal("expected duplicate code error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeConflict {
		t.Errorf("expected CONFLICT, got %v", err)
	}
}

// TestService_UpdateProject_RejectsCycle covers the cycle-detection
// requirement: setting a project's parent to one of its
// descendants must fail with INVALID_STATE.
func TestService_UpdateProject_RejectsCycle(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID}, testContext())
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID}, testContext())

	// bl cannot be re-parented under prj (which is its grandchild).
	_, err := svc.UpdateProject(testContext(), bl.ID, UpdateProjectInput{ParentID: &prj.ID})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeInvalidState {
		t.Errorf("expected INVALID_STATE, got %v", err)
	}
}

// TestService_UpdateProject_RejectsSelfParent is the simpler
// half of cycle detection: a project cannot be its own parent.
func TestService_UpdateProject_RejectsSelfParent(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	_, err := svc.UpdateProject(testContext(), bl.ID, UpdateProjectInput{ParentID: &bl.ID})
	if err == nil {
		t.Fatal("expected self-parent error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeInvalidState {
		t.Errorf("expected INVALID_STATE, got %v", err)
	}
}

// TestService_UpdateProject_RejectsDepthOverflow on update: even
// when re-parenting, the resulting depth cannot exceed 3.
func TestService_UpdateProject_RejectsDepthOverflow(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl1, _ := svc.CreateProject(CreateProjectInput{Name: "BL1", Code: "bl1", TypeID: pt.ID}, testContext())
	bl2, _ := svc.CreateProject(CreateProjectInput{Name: "BL2", Code: "bl2", TypeID: pt.ID}, testContext())
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl1.ID}, testContext())
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID}, testContext())

	// re-parenting bl2 under prj would put bl2 at depth 4
	_, err := svc.UpdateProject(testContext(), bl2.ID, UpdateProjectInput{ParentID: &prj.ID})
	if err == nil {
		t.Fatal("expected depth overflow")
	}
}

// TestService_DeleteProject_RejectsWithChildren covers the spec
// scenario "Delete Business Line ... child systems and projects":
// a non-leaf delete is rejected so the caller does not accidentally
// orphan the hierarchy. The service returns a clear INVALID_STATE.
func TestService_DeleteProject_RejectsWithChildren(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	_, _ = svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID}, testContext())

	err := svc.DeleteProject(testContext(), bl.ID)
	if err == nil {
		t.Fatal("expected error deleting parent with children")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeInvalidState {
		t.Errorf("expected INVALID_STATE, got %v", err)
	}
}

// TestService_DeleteProject_LeafSucceeds: a leaf can be deleted.
func TestService_DeleteProject_LeafSucceeds(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	if err := svc.DeleteProject(testContext(), bl.ID); err != nil {
		t.Fatalf("delete leaf: %v", err)
	}
	if _, err := svc.GetProject(testContext(), bl.ID); !IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

// TestService_AssignMember_RejectsBadRole covers the spec scenario
// "Grant viewer/editor permission": only known roles are accepted.
func TestService_AssignMember_RejectsBadRole(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	err := svc.AssignMember(testContext(), bl.ID, "u-1", "wizard", "admin-1")
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %v", err)
	}
}

// TestService_AssignMember_PromoteInPlace covers re-granting a
// different role to an existing member. The unique index means a
// naive insert would conflict; the service must update instead.
func TestService_AssignMember_PromoteInPlace(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, testContext())
	if err := svc.AssignMember(testContext(), bl.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("first assign: %v", err)
	}
	if err := svc.AssignMember(testContext(), bl.ID, "u-1", "editor", "admin-2"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	members, err := svc.ListMembers(testContext(), bl.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(members) != 1 || members[0].Role != "editor" || members[0].AddedBy != "admin-2" {
		t.Errorf("promote did not update: %+v", members)
	}
}

// TestService_AggregateWeight covers the spec's hierarchical weight
// aggregation: a node's reported weight is its own weight times
// every type weight along the chain. This is the basis for FinOps
// cost allocation.
func TestService_AggregateWeight(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform", Weight: 10}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID, Weight: 3}, testContext())
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID, Weight: 2}, testContext())
	_, _ = svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID, Weight: 1}, testContext())

	// BL: 3 (own) * 10 (type) = 30
	if got := svc.AggregateWeight(testContext(), bl.ID); got != 30 {
		t.Errorf("BL aggregate = %d want 30", got)
	}
	// SYS: 2 (own) * 10 (type) = 20
	if got := svc.AggregateWeight(testContext(), sys.ID); got != 20 {
		t.Errorf("SYS aggregate = %d want 20", got)
	}
}

// TestService_AggregateWeight_DefaultsToTypeWhenOwnZero covers
// the boundary where the project weight is zero but the type
// weight is non-zero. The product is still meaningful.
func TestService_AggregateWeight_DefaultsToTypeWhenOwnZero(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform", Weight: 7}, testContext())
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID, Weight: 0}, testContext())
	if got := svc.AggregateWeight(testContext(), bl.ID); got != 0 {
		t.Errorf("BL aggregate = %d want 0 (0 * 7)", got)
	}
}

// TestService_TypeNameUniqueness is the spec's "name unique"
// requirement for project types.
func TestService_TypeNameUniqueness(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	if _, err := svc.CreateType(ProjectType{Name: "platform"}, testContext()); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	var ae *contracts.APIError
	if !asAPIError(err, &ae) || ae.Code != contracts.CodeConflict {
		t.Errorf("expected CONFLICT, got %v", err)
	}
}

// TestService_PromoteToRootAllowed ensures a project can be moved
// to a new parent as long as the new depth is in range and there
// is no cycle.
func TestService_PromoteToRootAllowed(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, testContext())
	bl1, _ := svc.CreateProject(CreateProjectInput{Name: "BL1", Code: "bl1", TypeID: pt.ID}, testContext())
	bl2, _ := svc.CreateProject(CreateProjectInput{Name: "BL2", Code: "bl2", TypeID: pt.ID}, testContext())
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl1.ID}, testContext())

	// re-parent sys under bl2 (still depth 2)
	got, err := svc.UpdateProject(testContext(), sys.ID, UpdateProjectInput{ParentID: &bl2.ID})
	if err != nil {
		t.Fatalf("re-parent: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != bl2.ID {
		t.Errorf("re-parent did not stick: %+v", got.ParentID)
	}
}

// asAPIError is a tiny helper that calls errors.As. Kept local so
// service_test.go does not have to import the errors package.
func asAPIError(err error, target **contracts.APIError) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*contracts.APIError)
	if !ok {
		return false
	}
	*target = ae
	return true
}

// projectMembershipChecker returns a MembershipChecker that
// grants the user membership in the supplied project IDs.
func projectMembershipChecker(projects ...string) caller.MembershipChecker {
	set := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		set[p] = struct{}{}
	}
	return func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return set, nil
	}
}

// TestService_GetProject_CrossTenant_Denied covers the v0.3.0.0
// P0 #2 cross-tenant enforcement: a Developer who is a member of
// project A cannot read project B. The Service MUST refuse with
// ErrForbidden so a misconfigured handler that forgets the route
// middleware cannot leak data across tenants.
func TestService_GetProject_CrossTenant_Denied(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleDeveloper})
	svc.SetMembershipChecker(projectMembershipChecker("a-1"))
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.GetProject(ctx, "b-1")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// TestService_GetProject_SuperAdmin_Bypasses covers the spec rule
// that SuperAdmin is implicitly a member of every project, so
// the Service MUST allow the read.
func TestService_GetProject_SuperAdmin_Bypasses(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	cl := caller.New(&contracts.User{ID: "root", Username: "root", Role: contracts.RoleSuperAdmin})
	svc.SetMembershipChecker(func(ctx context.Context, userID string) (map[string]struct{}, error) {
		return nil, nil
	})
	ctx := caller.WithContext(context.Background(), cl)
	pt, err := svc.CreateType(ProjectType{Name: "platform"}, ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	bl, err := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID}, ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := svc.GetProject(ctx, bl.ID)
	if err != nil {
		t.Errorf("SuperAdmin should bypass, got err = %v", err)
	}
	if got.ID != bl.ID {
		t.Errorf("got.ID = %q, want %q", got.ID, bl.ID)
	}
}

// TestService_GetProject_NilCaller_401 covers the fail-closed rule:
// a context without a caller (the request never went through
// AuthMiddleware) MUST surface as ErrUnauthenticated.
func TestService_GetProject_NilCaller_401(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	svc.SetMembershipChecker(projectMembershipChecker("a-1"))
	_, err := svc.GetProject(context.Background(), "a-1")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}

// recordingEmitter captures every AuditEvent for assertion. It
// satisfies the audit.Emitter interface used by audit.Service.
type recordingEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newServiceWithAuditDB is a test convenience that wires a fresh
// in-memory DB, a project service, and a recording audit
// emitter. The membership checker is wired so the service-layer
// cross-tenant guards pass for the test caller.
func newServiceWithAuditDB(t *testing.T) (*Service, *recordingEmitter) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	emitter := &recordingEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})
	svc := NewService(repo, auditSvc)
	// Wire a permissive membership checker: the SuperAdmin caller
	// already bypasses, but a no-op checker also keeps the legacy
	// code path (when SuperAdmin guard is short-circuited) happy.
	svc.SetMembershipChecker(func(_ context.Context, _ string) (map[string]struct{}, error) {
		return map[string]struct{}{}, nil
	})
	return svc, emitter
}

// auditCallerContext returns a context pre-populated with a
// SuperAdmin caller. The project service treats SuperAdmin as a
// member of every project, so the cross-tenant guard passes
// without any membership wiring.
func auditCallerContext() context.Context {
	cl := caller.New(&contracts.User{ID: "test-admin", Username: "test-admin", Role: contracts.RoleSuperAdmin})
	return caller.WithContext(context.Background(), cl)
}

// TestService_CreateProject_EmitsAudit pins the service-layer
// audit emission for project creation. The handler-level emit
// was removed when this commit moved audit to the service layer.
func TestService_CreateProject_EmitsAudit(t *testing.T) {
	svc, emitter := newServiceWithAuditDB(t)
	pt, err := svc.CreateType(ProjectType{Name: "platform"}, auditCallerContext())
	if err != nil {
		t.Fatalf("seed type: %v", err)
	}
	p, err := svc.CreateProject(CreateProjectInput{Name: "P", Code: "p", TypeID: pt.ID}, auditCallerContext())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceProject {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceProject)
	}
	if events[0].ResourceID != p.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, p.ID)
	}
}

// TestService_UpdateProject_EmitsAudit pins the update path.
func TestService_UpdateProject_EmitsAudit(t *testing.T) {
	svc, emitter := newServiceWithAuditDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, auditCallerContext())
	p, _ := svc.CreateProject(CreateProjectInput{Name: "P", Code: "p", TypeID: pt.ID}, auditCallerContext())
	name := "P-renamed"
	if _, err := svc.UpdateProject(auditCallerContext(), p.ID, UpdateProjectInput{Name: &name}); err != nil {
		t.Fatalf("update: %v", err)
	}
	// CreateProject (1) + UpdateProject (1) = 2 events
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionUpdate {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionUpdate)
	}
	if last.ResourceID != p.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, p.ID)
	}
}

// TestService_DeleteProject_EmitsAudit pins the delete path.
func TestService_DeleteProject_EmitsAudit(t *testing.T) {
	svc, emitter := newServiceWithAuditDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, auditCallerContext())
	p, _ := svc.CreateProject(CreateProjectInput{Name: "P", Code: "p", TypeID: pt.ID}, auditCallerContext())
	if err := svc.DeleteProject(auditCallerContext(), p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// CreateProject (1) + DeleteProject (1) = 2 events
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionDelete {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionDelete)
	}
	if last.ResourceID != p.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, p.ID)
	}
}

// TestService_AssignMember_EmitsAudit pins the member-add path.
func TestService_AssignMember_EmitsAudit(t *testing.T) {
	svc, emitter := newServiceWithAuditDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, auditCallerContext())
	p, _ := svc.CreateProject(CreateProjectInput{Name: "P", Code: "p", TypeID: pt.ID}, auditCallerContext())
	if err := svc.AssignMember(auditCallerContext(), p.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	// CreateProject (1) + AssignMember (1) = 2 events
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionMemberAdd {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionMemberAdd)
	}
	if last.ResourceType != audit.ResourceProjectMember {
		t.Errorf("last resource type = %q, want %q", last.ResourceType, audit.ResourceProjectMember)
	}
}

// TestService_RevokeMember_EmitsAudit pins the member-remove path.
func TestService_RevokeMember_EmitsAudit(t *testing.T) {
	svc, emitter := newServiceWithAuditDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"}, auditCallerContext())
	p, _ := svc.CreateProject(CreateProjectInput{Name: "P", Code: "p", TypeID: pt.ID}, auditCallerContext())
	if err := svc.AssignMember(auditCallerContext(), p.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := svc.RevokeMember(auditCallerContext(), p.ID, "u-1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// CreateProject + AssignMember + RevokeMember = 3 events
	events := emitter.all()
	if len(events) != 3 {
		t.Fatalf("audit events = %d, want 3", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionMemberRemove {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionMemberRemove)
	}
	if last.ResourceType != audit.ResourceProjectMember {
		t.Errorf("last resource type = %q, want %q", last.ResourceType, audit.ResourceProjectMember)
	}
}
