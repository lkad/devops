// Package rbac implements the role-based access control layer for
// the DevOps Toolkit backend. The matrix, service, and middleware
// live together here because they are tightly coupled: the matrix
// is the data, the service is the decision logic, and the middleware
// is the HTTP enforcement point. The package depends only on
// pkg/contracts so it can be imported by any module without
// creating an import cycle.
package rbac

import "github.com/devops-toolkit/backend/pkg/contracts"

// Permission names a single, addressable capability the system can
// grant. New permissions are added by extending the const block and
// the RolePermissions table — no other code needs to change.
type Permission string

const (
	// PermissionViewDevices lets a caller read device lists and details.
	PermissionViewDevices Permission = "devices.view"
	// PermissionModifyConfig lets a caller PUT/POST device configuration.
	PermissionModifyConfig Permission = "devices.config.modify"
	// PermissionExecuteCommands lets a caller run ad-hoc commands on
	// a device (deploys, scripts, etc.).
	PermissionExecuteCommands Permission = "devices.execute"
	// PermissionRemoteRestart lets a caller restart a device. The
	// production-environment restriction for Operators is enforced
	// by the service layer, not by the matrix.
	PermissionRemoteRestart Permission = "devices.restart"

	// Physical-host module permissions. Read mirrors
	// PermissionViewDevices; write covers create / update / delete;
	// probe covers manual SSH check triggers; maintenance covers
	// enter / exit maintenance windows. Operators get probe +
	// maintenance; only SuperAdmin gets write on prod-class
	// hosts (gated in the service layer).
	PermissionViewPhysicalHosts    Permission = "physicalhost.view"
	PermissionWritePhysicalHosts   Permission = "physicalhost.write"
	PermissionProbePhysicalHost    Permission = "physicalhost.probe"
	PermissionMaintenancePhysical  Permission = "physicalhost.maintenance"
	PermissionViewAuditLog         Permission = "audit.view"
)

// allPermissions is the canonical ordered list of permissions. The
// matrix and the tests both iterate it; the slice form is more
// readable than building a map just to range over keys.
var allPermissions = []Permission{
	PermissionViewDevices,
	PermissionModifyConfig,
	PermissionExecuteCommands,
	PermissionRemoteRestart,
	PermissionViewPhysicalHosts,
	PermissionWritePhysicalHosts,
	PermissionProbePhysicalHost,
	PermissionMaintenancePhysical,
	PermissionViewAuditLog,
}

// AllPermissions returns a copy of the permission catalog. Callers
// may iterate freely; mutating the result does not affect the table.
func AllPermissions() []Permission {
	out := make([]Permission, len(allPermissions))
	copy(out, allPermissions)
	return out
}

// RolePermissions is the declarative role → permission mapping.
// It is the single source of truth for the matrix table in the
// rbac-permissions spec. RoleProjectAdmin is intentionally absent
// because it is a project-scoped role, not a global one — project
// permissions are evaluated by HasPermissionInProject.
//
// The matrix is intentionally permissive at the global level:
// Operator has PermissionRemoteRestart, with the understanding that
// the service layer adds a production-environment check before the
// action runs. Splitting the static role from the runtime context
// is what makes the matrix reusable across endpoints.
var RolePermissions = map[contracts.Role][]Permission{
	contracts.RoleSuperAdmin: {
		PermissionViewDevices,
		PermissionModifyConfig,
		PermissionExecuteCommands,
		PermissionRemoteRestart,
		PermissionViewPhysicalHosts,
		PermissionWritePhysicalHosts,
		PermissionProbePhysicalHost,
		PermissionMaintenancePhysical,
		PermissionViewAuditLog,
	},
	contracts.RoleOperator: {
		PermissionViewDevices,
		PermissionModifyConfig,
		PermissionExecuteCommands,
		// PermissionRemoteRestart is denied: production-restart
		// gating happens in the service layer.
		PermissionViewPhysicalHosts,
		PermissionWritePhysicalHosts,
		PermissionProbePhysicalHost,
		PermissionMaintenancePhysical,
		PermissionViewAuditLog,
	},
	contracts.RoleDeveloper: {
		PermissionViewDevices,
		PermissionViewPhysicalHosts,
	},
	contracts.RoleAuditor: {
		PermissionViewDevices,
		PermissionViewPhysicalHosts,
		PermissionViewAuditLog,
	},
}

// RoleHasPermission reports whether role r grants permission p.
// Unknown roles and unknown permissions both return false; this
// keeps the function total so the middleware never has to special-
// case a missing entry.
func RoleHasPermission(r contracts.Role, p Permission) bool {
	perms, ok := RolePermissions[r]
	if !ok {
		return false
	}
	for _, candidate := range perms {
		if candidate == p {
			return true
		}
	}
	return false
}
