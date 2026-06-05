// Package rbac tests cover the role × permission matrix and the
// middleware chain. Every test is written from the spec
// (openspec/specs/rbac-permissions/spec.md) — a scenario block in
// the spec maps to a single test function here.
package rbac

import (
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestMatrix_AllRolesHaveViewPermission is the spec's "Auditor/Developer
// have read access" scenario extended: every authenticated role can
// see devices (the matrix is the source of truth, not the role rank).
func TestMatrix_AllRolesHaveViewPermission(t *testing.T) {
	// GIVEN the four global roles from the matrix
	// WHEN we look up View Devices
	// THEN every role including Auditor and Developer can view
	roles := []contracts.Role{
		contracts.RoleSuperAdmin,
		contracts.RoleOperator,
		contracts.RoleDeveloper,
		contracts.RoleAuditor,
	}
	for _, r := range roles {
		if !RoleHasPermission(r, PermissionViewDevices) {
			t.Errorf("role %s should have %s", r, PermissionViewDevices)
		}
	}
}

// TestMatrix_AuditorIsReadOnly is the spec scenario "Auditor has
// read-only access": every write permission must be denied for Auditor.
func TestMatrix_AuditorIsReadOnly(t *testing.T) {
	// GIVEN Auditor
	// WHEN we check any write permission
	// THEN it is denied
	writes := []Permission{
		PermissionModifyConfig,
		PermissionExecuteCommands,
		PermissionRemoteRestart,
	}
	for _, p := range writes {
		if RoleHasPermission(contracts.RoleAuditor, p) {
			t.Errorf("Auditor must not have %s", p)
		}
	}
}

// TestMatrix_DeveloperIsReadOnly is the spec scenario "Developer
// denied modify operations": Developers can read but not write.
func TestMatrix_DeveloperIsReadOnly(t *testing.T) {
	writes := []Permission{
		PermissionModifyConfig,
		PermissionExecuteCommands,
		PermissionRemoteRestart,
	}
	for _, p := range writes {
		if RoleHasPermission(contracts.RoleDeveloper, p) {
			t.Errorf("Developer must not have %s", p)
		}
	}
}

// TestMatrix_OperatorHasDeployAndConfig is the spec scenario
// "Operator has deploy and config permissions": Operator can
// modify config and execute commands (deploy).
func TestMatrix_OperatorHasDeployAndConfig(t *testing.T) {
	for _, p := range []Permission{PermissionModifyConfig, PermissionExecuteCommands} {
		if !RoleHasPermission(contracts.RoleOperator, p) {
			t.Errorf("Operator should have %s", p)
		}
	}
}

// TestMatrix_OperatorCannotRestart is the spec scenario "Operator
// restricted on production restart": the matrix denies Operator
// the remote-restart permission outright; the production-vs-not
// distinction is enforced at the service layer.
func TestMatrix_OperatorCannotRestart(t *testing.T) {
	if RoleHasPermission(contracts.RoleOperator, PermissionRemoteRestart) {
		t.Error("Operator should not have PermissionRemoteRestart in the matrix")
	}
}

// TestMatrix_SuperAdminHasAll is the spec scenario "SuperAdmin has
// all permissions": every permission in the catalog is granted to
// SuperAdmin.
func TestMatrix_SuperAdminHasAll(t *testing.T) {
	for _, p := range AllPermissions() {
		if !RoleHasPermission(contracts.RoleSuperAdmin, p) {
			t.Errorf("SuperAdmin should have %s", p)
		}
	}
}

// TestMatrix_RolePermissionsTableMatchesSpec is the spec's full
// permission matrix expressed as one table-driven test. The
// expected column is taken directly from the spec's matrix table.
func TestMatrix_RolePermissionsTableMatchesSpec(t *testing.T) {
	// columns: View, ModifyConfig, ExecuteCommands, RemoteRestart
	cases := []struct {
		role  contracts.Role
		view  bool
		mod   bool
		exec  bool
		restart bool
	}{
		{contracts.RoleAuditor,    true, false, false, false},
		{contracts.RoleDeveloper,  true, false, false, false},
		{contracts.RoleOperator,   true, true,  true,  false},
		{contracts.RoleSuperAdmin, true, true,  true,  true},
	}
	for _, c := range cases {
		got := []bool{
			RoleHasPermission(c.role, PermissionViewDevices),
			RoleHasPermission(c.role, PermissionModifyConfig),
			RoleHasPermission(c.role, PermissionExecuteCommands),
			RoleHasPermission(c.role, PermissionRemoteRestart),
		}
		want := []bool{c.view, c.mod, c.exec, c.restart}
		for i, p := range []Permission{
			PermissionViewDevices,
			PermissionModifyConfig,
			PermissionExecuteCommands,
			PermissionRemoteRestart,
		} {
			if got[i] != want[i] {
				t.Errorf("role=%s perm=%s got=%v want=%v", c.role, p, got[i], want[i])
			}
		}
	}
}

// TestMatrix_UnknownRoleHasNothing guards against typos and
// future role additions — a role not in the table must not
// silently gain permissions.
func TestMatrix_UnknownRoleHasNothing(t *testing.T) {
	for _, p := range AllPermissions() {
		if RoleHasPermission(contracts.Role("ghost"), p) {
			t.Errorf("unknown role 'ghost' must not have %s", p)
		}
	}
}
