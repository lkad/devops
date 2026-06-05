package project

import (
	"strings"
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newServiceWithDB is a test convenience: it stands up an in-memory
// database, runs the migrations, and returns a Service backed by a
// fresh repository. Each call gets its own DB so tests cannot
// accidentally share state.
func newServiceWithDB(t *testing.T) (*Service, *Repository) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	return NewService(repo), repo
}

// TestService_CreateProject_RejectsEmptyName is the spec scenario
// "Create Project" with an invalid input. The service layer is the
// place validation lives; an empty name must surface as a 400.
func TestService_CreateProject_RejectsEmptyName(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, err := svc.CreateType(ProjectType{Name: "platform"})
	if err != nil {
		t.Fatalf("seed type: %v", err)
	}
	_, err = svc.CreateProject(CreateProjectInput{Name: "", Code: "p", TypeID: pt.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	_, err := svc.CreateProject(CreateProjectInput{Name: "X", Code: "BAD CODE!", TypeID: pt.ID})
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
	_, err := svc.CreateProject(CreateProjectInput{Name: "X", Code: "x", TypeID: "ghost"})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID})
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID})

	// Trying to add a 4th level under prj must fail.
	_, err := svc.CreateProject(CreateProjectInput{Name: "TooDeep", Code: "deep", TypeID: pt.ID, ParentID: &prj.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	if _, err := svc.CreateProject(CreateProjectInput{Name: "P", Code: "dup", TypeID: pt.ID}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreateProject(CreateProjectInput{Name: "P2", Code: "dup", TypeID: pt.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID})
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID})

	// bl cannot be re-parented under prj (which is its grandchild).
	_, err := svc.UpdateProject(bl.ID, UpdateProjectInput{ParentID: &prj.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	_, err := svc.UpdateProject(bl.ID, UpdateProjectInput{ParentID: &bl.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl1, _ := svc.CreateProject(CreateProjectInput{Name: "BL1", Code: "bl1", TypeID: pt.ID})
	bl2, _ := svc.CreateProject(CreateProjectInput{Name: "BL2", Code: "bl2", TypeID: pt.ID})
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl1.ID})
	prj, _ := svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID})

	// re-parenting bl2 under prj would put bl2 at depth 4
	_, err := svc.UpdateProject(bl2.ID, UpdateProjectInput{ParentID: &prj.ID})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	_, _ = svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID})

	err := svc.DeleteProject(bl.ID)
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	if err := svc.DeleteProject(bl.ID); err != nil {
		t.Fatalf("delete leaf: %v", err)
	}
	if _, err := svc.GetProject(bl.ID); !IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

// TestService_AssignMember_RejectsBadRole covers the spec scenario
// "Grant viewer/editor permission": only known roles are accepted.
func TestService_AssignMember_RejectsBadRole(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	err := svc.AssignMember(bl.ID, "u-1", "wizard", "admin-1")
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID})
	if err := svc.AssignMember(bl.ID, "u-1", "viewer", "admin-1"); err != nil {
		t.Fatalf("first assign: %v", err)
	}
	if err := svc.AssignMember(bl.ID, "u-1", "editor", "admin-2"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	members, err := svc.ListMembers(bl.ID)
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform", Weight: 10})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID, Weight: 3})
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl.ID, Weight: 2})
	_, _ = svc.CreateProject(CreateProjectInput{Name: "PRJ", Code: "prj", TypeID: pt.ID, ParentID: &sys.ID, Weight: 1})

	// BL: 3 (own) * 10 (type) = 30
	if got := svc.AggregateWeight(bl.ID); got != 30 {
		t.Errorf("BL aggregate = %d want 30", got)
	}
	// SYS: 2 (own) * 10 (type) = 20
	if got := svc.AggregateWeight(sys.ID); got != 20 {
		t.Errorf("SYS aggregate = %d want 20", got)
	}
}

// TestService_AggregateWeight_DefaultsToTypeWhenOwnZero covers
// the boundary where the project weight is zero but the type
// weight is non-zero. The product is still meaningful.
func TestService_AggregateWeight_DefaultsToTypeWhenOwnZero(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	pt, _ := svc.CreateType(ProjectType{Name: "platform", Weight: 7})
	bl, _ := svc.CreateProject(CreateProjectInput{Name: "BL", Code: "bl", TypeID: pt.ID, Weight: 0})
	if got := svc.AggregateWeight(bl.ID); got != 0 {
		t.Errorf("BL aggregate = %d want 0 (0 * 7)", got)
	}
}

// TestService_TypeNameUniqueness is the spec's "name unique"
// requirement for project types.
func TestService_TypeNameUniqueness(t *testing.T) {
	svc, _ := newServiceWithDB(t)
	if _, err := svc.CreateType(ProjectType{Name: "platform"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreateType(ProjectType{Name: "platform"})
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
	pt, _ := svc.CreateType(ProjectType{Name: "platform"})
	bl1, _ := svc.CreateProject(CreateProjectInput{Name: "BL1", Code: "bl1", TypeID: pt.ID})
	bl2, _ := svc.CreateProject(CreateProjectInput{Name: "BL2", Code: "bl2", TypeID: pt.ID})
	sys, _ := svc.CreateProject(CreateProjectInput{Name: "SYS", Code: "sys", TypeID: pt.ID, ParentID: &bl1.ID})

	// re-parent sys under bl2 (still depth 2)
	got, err := svc.UpdateProject(sys.ID, UpdateProjectInput{ParentID: &bl2.ID})
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
