package contracts

import "testing"

func TestRole_HasPermission(t *testing.T) {
	// GIVEN a SuperAdmin
	// WHEN checking any role
	// THEN it has permission
	admin := RoleSuperAdmin
	for _, r := range []Role{RoleSuperAdmin, RoleOperator, RoleDeveloper, RoleAuditor, RoleProjectAdmin} {
		if !admin.HasPermission(r) {
			t.Errorf("SuperAdmin should have permission over %s", r)
		}
	}
}

func TestRole_AuditorIsReadOnly(t *testing.T) {
	// GIVEN an Auditor
	// WHEN checking write-style roles
	// THEN it does not have permission
	auditor := RoleAuditor
	if auditor.HasPermission(RoleSuperAdmin) {
		t.Error("Auditor should not match SuperAdmin")
	}
	if auditor.HasPermission(RoleOperator) {
		t.Error("Auditor should not match Operator")
	}
}

func TestRole_UnknownIsUnprivileged(t *testing.T) {
	// GIVEN an unrecognized role
	// WHEN checking it
	// THEN HasPermission returns false for everything except itself
	unknown := Role("mystery")
	if unknown.HasPermission(RoleSuperAdmin) {
		t.Error("unknown role should not have SuperAdmin")
	}
}

func TestJWTClaims_ExpiresAtInFuture(t *testing.T) {
	// GIVEN a JWT claim with an expiry
	// WHEN we check whether it is expired
	// THEN unexpired tokens are accepted
	c := &JWTClaims{Username: "u", Role: RoleSuperAdmin, ExpiresAt: 9999999999}
	if c.IsExpired() {
		t.Error("future-expired token marked expired")
	}
}

func TestProjectRole_Hierarchy(t *testing.T) {
	// GIVEN a project admin
	// WHEN checking lower roles
	// THEN it has permission over them
	admin := ProjectRoleAdmin
	if !admin.HasPermission(ProjectRoleEditor) {
		t.Error("admin should have permission over editor")
	}
	if !admin.HasPermission(ProjectRoleViewer) {
		t.Error("admin should have permission over viewer")
	}
	// GIVEN a viewer
	// WHEN checking higher roles
	// THEN it does not have permission
	viewer := ProjectRoleViewer
	if viewer.HasPermission(ProjectRoleAdmin) {
		t.Error("viewer should not have permission over admin")
	}
}
