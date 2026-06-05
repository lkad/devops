package rbac

import (
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestService_HasPermission_GreenForAllowed is the spec scenario
// "SuperAdmin has all permissions" + "Operator has deploy and
// config permissions": the service allows a user whose role grants
// the permission.
func TestService_HasPermission_GreenForAllowed(t *testing.T) {
	svc := NewService()
	cases := []struct {
		name string
		user *contracts.User
		perm Permission
	}{
		{"superadmin can modify", &contracts.User{ID: "u1", Role: contracts.RoleSuperAdmin}, PermissionModifyConfig},
		{"operator can execute", &contracts.User{ID: "u2", Role: contracts.RoleOperator}, PermissionExecuteCommands},
		{"auditor can view", &contracts.User{ID: "u3", Role: contracts.RoleAuditor}, PermissionViewDevices},
		{"developer can view", &contracts.User{ID: "u4", Role: contracts.RoleDeveloper}, PermissionViewDevices},
	}
	for _, c := range cases {
		if !svc.HasPermission(c.user, c.perm) {
			t.Errorf("%s: expected allow for role=%s perm=%s", c.name, c.user.Role, c.perm)
		}
	}
}

// TestService_HasPermission_DeniesAuditorWrite is the spec
// scenario "Auditor has read-only access": a write permission is
// denied for an Auditor.
func TestService_HasPermission_DeniesAuditorWrite(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleAuditor}
	if svc.HasPermission(u, PermissionModifyConfig) {
		t.Error("Auditor should be denied PermissionModifyConfig")
	}
	if svc.HasPermission(u, PermissionExecuteCommands) {
		t.Error("Auditor should be denied PermissionExecuteCommands")
	}
	if svc.HasPermission(u, PermissionRemoteRestart) {
		t.Error("Auditor should be denied PermissionRemoteRestart")
	}
}

// TestService_HasPermission_DeniesDeveloperWrite is the spec
// scenario "Developer denied modify operations": a Developer
// cannot modify config.
func TestService_HasPermission_DeniesDeveloperWrite(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleDeveloper}
	if svc.HasPermission(u, PermissionModifyConfig) {
		t.Error("Developer should be denied PermissionModifyConfig")
	}
}

// TestService_HasPermission_NilUserIsSafe covers the degenerate
// input — a nil pointer must not panic and must deny. This is
// the kind of guard the middleware relies on when an upstream
// auth step fails to populate the user.
func TestService_HasPermission_NilUserIsSafe(t *testing.T) {
	svc := NewService()
	if svc.HasPermission(nil, PermissionViewDevices) {
		t.Error("nil user should not be allowed")
	}
}

// TestService_HasPermission_ProjectAdminAlone is the spec's
// per-project role story: ProjectAdmin is a project-scoped role.
// At the global level, it grants nothing (the matrix has no
// entry for it). Project-scoped checks use HasPermissionInProject.
func TestService_HasPermission_ProjectAdminAlone(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleProjectAdmin}
	if svc.HasPermission(u, PermissionViewDevices) {
		t.Error("ProjectAdmin alone should not grant global PermissionViewDevices")
	}
	if svc.HasPermission(u, PermissionModifyConfig) {
		t.Error("ProjectAdmin alone should not grant global PermissionModifyConfig")
	}
}

// TestService_HasPermissionInProject_AdminGrantsAll is the spec
// scenario "Permission inheritance: BusinessLine → System →
// Project": a project admin has the full project permission set.
func TestService_HasPermissionInProject_AdminGrantsAll(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleOperator, ProjectRole: contracts.ProjectRoleAdmin}
	for _, p := range AllPermissions() {
		if !svc.HasPermissionInProject(u, "any-project", p) {
			t.Errorf("project admin should have %s in project", p)
		}
	}
}

// TestService_HasPermissionInProject_EditorCannotRestart is the
// spec scenario "Operator restricted on production restart"
// expressed in project terms: a project editor cannot restart
// devices at the project level (only admins can).
func TestService_HasPermissionInProject_EditorCannotRestart(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleAuditor, ProjectRole: contracts.ProjectRoleEditor}
	if svc.HasPermissionInProject(u, "proj-1", PermissionRemoteRestart) {
		t.Error("project editor should not have PermissionRemoteRestart")
	}
	if !svc.HasPermissionInProject(u, "proj-1", PermissionViewDevices) {
		t.Error("project editor should have PermissionViewDevices")
	}
}

// TestService_HasPermissionInProject_ViewerReadOnly is the spec
// "Auditor has read-only access" / "Developer denied modify"
// project-tier equivalent: a project viewer can only read.
func TestService_HasPermissionInProject_ViewerReadOnly(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleSuperAdmin, ProjectRole: contracts.ProjectRoleViewer}
	writes := []Permission{PermissionModifyConfig, PermissionExecuteCommands, PermissionRemoteRestart}
	for _, p := range writes {
		if svc.HasPermissionInProject(u, "proj-1", p) {
			t.Errorf("project viewer should not have %s even with SuperAdmin global role", p)
		}
	}
	if !svc.HasPermissionInProject(u, "proj-1", PermissionViewDevices) {
		t.Error("project viewer should have PermissionViewDevices")
	}
}

// TestService_HasPermissionInProject_NoRoleDenies covers the
// boundary case: a user with no global role and no project role
// must be denied everything in a project.
func TestService_HasPermissionInProject_NoRoleDenies(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.Role("unknown"), ProjectRole: ""}
	if svc.HasPermissionInProject(u, "proj-1", PermissionViewDevices) {
		t.Error("user with no role should be denied project view")
	}
}

// TestService_CanRestartInEnvironment_OperatorProd is the spec
// scenario "Operator restricted on production restart": even
// though Operators are denied PermissionRemoteRestart by the
// matrix, this helper exists so the service layer can compose
// the role check with the runtime environment flag. The check
// is intentionally strict: production devices are off-limits
// to anyone below SuperAdmin.
func TestService_CanRestartInEnvironment_OperatorProd(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleOperator}
	if svc.CanRestartInEnvironment(u, true) {
		t.Error("Operator should not be able to restart production devices")
	}
}

// TestService_CanRestartInEnvironment_SuperAdminProd is the
// companion scenario: SuperAdmin can restart anything, including
// production devices.
func TestService_CanRestartInEnvironment_SuperAdminProd(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleSuperAdmin}
	if !svc.CanRestartInEnvironment(u, true) {
		t.Error("SuperAdmin should be able to restart production devices")
	}
}

// TestService_CanRestartInEnvironment_OperatorNonProd: per the
// spec's matrix note ("Operator can restart non-production
// devices only"), the non-production case is allowed when the
// matrix permits it. Today the matrix denies Operator the
// permission outright, so this test pins the current contract:
// non-prod restart is still denied until the matrix changes.
func TestService_CanRestartInEnvironment_OperatorNonProd(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleOperator}
	if svc.CanRestartInEnvironment(u, false) {
		t.Error("Operator is denied PermissionRemoteRestart in the matrix; non-prod must also be denied")
	}
}

// TestService_HasAccessByLabel_Match is the spec scenario
// "User accesses device with matching label": a user whose
// group matches the device's label group is allowed.
func TestService_HasAccessByLabel_Match(t *testing.T) {
	svc := NewService()
	userGroups := []string{"team-a"}
	deviceLabels := []string{"team-a", "team-b"}
	if !svc.HasAccessByLabel(userGroups, deviceLabels) {
		t.Error("matching label should grant access")
	}
}

// TestService_HasAccessByLabel_NoMatch is the spec scenario
// "User accesses device without matching label": a user with
// no overlapping group is denied.
func TestService_HasAccessByLabel_NoMatch(t *testing.T) {
	svc := NewService()
	userGroups := []string{"team-c"}
	deviceLabels := []string{"team-a", "team-b"}
	if svc.HasAccessByLabel(userGroups, deviceLabels) {
		t.Error("non-matching label should deny access")
	}
}

// TestService_HasAccessByLabel_SuperAdminBypass: SuperAdmin is
// the global escape hatch and is always allowed regardless of
// labels. The helper takes userGroups so callers that want the
// bypass must explicitly opt in by passing role=SuperAdmin.
func TestService_HasAccessByLabel_SuperAdminBypass(t *testing.T) {
	svc := NewService()
	userGroups := []string{}
	deviceLabels := []string{"team-a"}
	if !svc.HasAccessByLabelForRole(contracts.RoleSuperAdmin, userGroups, deviceLabels) {
		t.Error("SuperAdmin should bypass label check")
	}
}

// TestService_InheritedLabels_ChildInheritsParent is the spec
// scenario "Child group inherits parent labels": a device in
// a child group inherits the parent's labels.
func TestService_InheritedLabels_ChildInheritsParent(t *testing.T) {
	got := InheritedLabels([]string{"team-a", "team-a/subteam-1"})
	want := map[string]bool{
		"team-a":           true,
		"team-a/subteam-1": true,
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected inherited label %q", g)
		}
		delete(want, g)
	}
	if len(want) > 0 {
		t.Errorf("missing inherited labels: %v", want)
	}
}

// TestService_InheritedLabels_Empty: an empty hierarchy yields
// no labels. The helper is total so the middleware can call it
// without nil-checking.
func TestService_InheritedLabels_Empty(t *testing.T) {
	got := InheritedLabels(nil)
	if len(got) != 0 {
		t.Errorf("expected no labels for nil hierarchy, got %v", got)
	}
}

// TestService_OperatorProdModification is the spec scenario
// "Operator restricted on prod devices": the env-aware
// modification check denies Operators on production.
func TestService_OperatorProdModification(t *testing.T) {
	svc := NewService()
	u := &contracts.User{ID: "u", Role: contracts.RoleOperator}
	if svc.CanModifyInEnvironment(u, true) {
		t.Error("Operator should not be able to modify production devices")
	}
	if !svc.CanModifyInEnvironment(u, false) {
		t.Error("Operator should be able to modify non-production devices")
	}
}
