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

	// PermissionWriteDevices covers POST/PUT/DELETE on the device
	// management surface (create / replace / delete). Distinct
	// from PermissionModifyConfig (which is the *config* modify
	// path used by the device actions API).
	PermissionWriteDevices Permission = "devices.write"
	// PermissionManageDeviceGroups covers the /device-groups
	// and /configuration-templates CRUD surfaces.
	PermissionManageDeviceGroups Permission = "device_groups.write"
	// PermissionManageConfigurationTemplates covers
	// /configuration-templates CRUD. Aliases
	// PermissionManageDeviceGroups where the read and write
	// surfaces overlap; kept separate so a future spec tweak
	// can grant template-write without granting group-write.
	PermissionManageConfigurationTemplates Permission = "templates.write"

	// PermissionViewProjects / PermissionWriteProjects cover
	// the project-hierarchy module's read and write surfaces.
	// Members management maps onto Write.
	PermissionViewProjects  Permission = "projects.view"
	PermissionWriteProjects Permission = "projects.write"

	// PermissionManageHostProjectLinks covers the
	// host ↔ project link table (link / unlink / bulk-link).
	PermissionManageHostProjectLinks Permission = "hostproject.write"

	// PermissionRunDiscovery covers POST /discovery/runs and
	// POST .../promote; the read surface reuses
	// PermissionViewDevices for host discovery output.
	PermissionRunDiscovery Permission = "discovery.run"

	// PermissionManageK8sClusters covers CRUD on /k8s/clusters
	// (create / replace / delete / probe). The per-cluster
	// read paths (pods, deployments, services) use the
	// view permission.
	PermissionManageK8sClusters Permission = "k8s.clusters.write"
	// PermissionViewK8sResources covers the read surface for
	// pods / deployments / services / logs against a
	// registered cluster.
	PermissionViewK8sResources Permission = "k8s.resources.view"
	// PermissionExecK8sPod covers the SPDY pod-exec endpoint
	// (POST .../pods/:pod/exec). The most dangerous k8s
	// surface; only SuperAdmin / Operator get it.
	PermissionExecK8sPod Permission = "k8s.pods.exec"
	// PermissionViewK8sPodLogs covers the per-cluster log
	// query (GET .../namespaces/:ns/logs) and the
	// logstream package's /logs, /stream, /sse endpoints.
	PermissionViewK8sPodLogs Permission = "k8s.logs.view"

	// PermissionManagePipelines covers pipeline CRUD plus
	// trigger / cancel. Stats / phases / run-listing reuse
	// the view permission.
	PermissionManagePipelines Permission = "pipelines.write"
	// PermissionViewPipelines covers GET on pipelines, runs,
	// stats, and phases. Read is intentionally broad — every
	// role except a hypothetical write-only role needs to
	// see pipeline state.
	PermissionViewPipelines Permission = "pipelines.view"

	// PermissionManageServiceCatalog covers CRUD on the
	// microservice catalog; the /health rollup is read-only
	// and reuses the view permission.
	PermissionManageServiceCatalog Permission = "services.write"
	// PermissionViewServiceCatalog covers the GET /services
	// surface and the /:id/health rollup.
	PermissionViewServiceCatalog Permission = "services.view"

	// Physical-host module permissions. Read mirrors
	// PermissionViewDevices; write covers create / update / delete;
	// probe covers manual SSH check triggers; maintenance covers
	// enter / exit maintenance windows. Operators get probe +
	// maintenance; only SuperAdmin gets write on prod-class
	// hosts (gated in the service layer).
	PermissionViewPhysicalHosts   Permission = "physicalhost.view"
	PermissionWritePhysicalHosts  Permission = "physicalhost.write"
	PermissionProbePhysicalHost   Permission = "physicalhost.probe"
	PermissionMaintenancePhysical Permission = "physicalhost.maintenance"
	PermissionViewAuditLog        Permission = "audit.view"
	// PermissionViewAuditLogProject covers GET /audit for a
	// per-tenant scope. The /audit handler routes
	// "ScopedAuditor"-role callers through ListForCaller's
	// ProjectIDsIn filter so they only see audit events for
	// projects they are a member of. Distinct from
	// PermissionViewAuditLog which gives full read access.
	PermissionViewAuditLogProject Permission = "audit.view.project"

	// PermissionViewLogs covers GET on the log-aggregation
	// surface (capabilities, query, streams). The retention /
	// saved-filters / alert-rules mutations live under
	// PermissionWriteLogs.
	PermissionViewLogs Permission = "logs.view"
	// PermissionWriteLogs covers the dev-only /logs/_test/echo
	// endpoint and the retention / saved-filters / alert-rules
	// write surfaces. Auditor / Developer do not get it.
	PermissionWriteLogs Permission = "logs.write"

	// PermissionViewMetrics covers the read surface for the
	// metrics module. The POST /metrics ingest path is gated
	// by PermissionWriteMetrics so a read-only role can
	// browse the catalog without being able to spoof series.
	PermissionViewMetrics  Permission = "metrics.view"
	PermissionWriteMetrics Permission = "metrics.write"

	// PermissionViewAlerts covers the GET /alerts surface
	// (list / stats / history / channels). The mutation
	// paths (acknowledge / resolve / create / delete /
	// channel CRUD) live under PermissionWriteAlerts.
	PermissionViewAlerts  Permission = "alerts.view"
	PermissionWriteAlerts Permission = "alerts.write"

	// PermissionViewDiscovery covers GET on /discovery/runs
	// (the read surface for the discovery module).
	PermissionViewDiscovery Permission = "discovery.view"
)

// allPermissions is the canonical ordered list of permissions. The
// matrix and the tests both iterate it; the slice form is more
// readable than building a map just to range over keys.
var allPermissions = []Permission{
	PermissionViewDevices,
	PermissionModifyConfig,
	PermissionExecuteCommands,
	PermissionRemoteRestart,
	PermissionWriteDevices,
	PermissionManageDeviceGroups,
	PermissionManageConfigurationTemplates,
	PermissionViewProjects,
	PermissionWriteProjects,
	PermissionManageHostProjectLinks,
	PermissionViewDiscovery,
	PermissionRunDiscovery,
	PermissionManageK8sClusters,
	PermissionViewK8sResources,
	PermissionExecK8sPod,
	PermissionViewK8sPodLogs,
	PermissionViewPipelines,
	PermissionManagePipelines,
	PermissionViewServiceCatalog,
	PermissionManageServiceCatalog,
	PermissionViewPhysicalHosts,
	PermissionWritePhysicalHosts,
	PermissionProbePhysicalHost,
	PermissionMaintenancePhysical,
	PermissionViewAuditLog,
	PermissionViewLogs,
	PermissionWriteLogs,
	PermissionViewMetrics,
	PermissionWriteMetrics,
	PermissionViewAlerts,
	PermissionWriteAlerts,
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
		PermissionWriteDevices,
		PermissionManageDeviceGroups,
		PermissionManageConfigurationTemplates,
		PermissionViewProjects,
		PermissionWriteProjects,
		PermissionManageHostProjectLinks,
		PermissionViewDiscovery,
		PermissionRunDiscovery,
		PermissionManageK8sClusters,
		PermissionViewK8sResources,
		PermissionExecK8sPod,
		PermissionViewK8sPodLogs,
		PermissionViewPipelines,
		PermissionManagePipelines,
		PermissionViewServiceCatalog,
		PermissionManageServiceCatalog,
		PermissionViewPhysicalHosts,
		PermissionWritePhysicalHosts,
		PermissionProbePhysicalHost,
		PermissionMaintenancePhysical,
		PermissionViewAuditLog,
		PermissionViewLogs,
		PermissionWriteLogs,
		PermissionViewMetrics,
		PermissionWriteMetrics,
		PermissionViewAlerts,
		PermissionWriteAlerts,
	},
	contracts.RoleOperator: {
		PermissionViewDevices,
		PermissionModifyConfig,
		PermissionExecuteCommands,
		// PermissionRemoteRestart is denied: production-restart
		// gating happens in the service layer.
		PermissionWriteDevices,
		PermissionManageDeviceGroups,
		PermissionManageConfigurationTemplates,
		PermissionViewProjects,
		PermissionWriteProjects,
		PermissionManageHostProjectLinks,
		PermissionViewDiscovery,
		PermissionRunDiscovery,
		PermissionManageK8sClusters,
		PermissionViewK8sResources,
		PermissionExecK8sPod,
		PermissionViewK8sPodLogs,
		PermissionViewPipelines,
		PermissionManagePipelines,
		PermissionViewServiceCatalog,
		PermissionManageServiceCatalog,
		PermissionViewPhysicalHosts,
		PermissionWritePhysicalHosts,
		PermissionProbePhysicalHost,
		PermissionMaintenancePhysical,
		PermissionViewAuditLog,
		PermissionViewLogs,
		PermissionWriteLogs,
		PermissionViewMetrics,
		PermissionWriteMetrics,
		PermissionViewAlerts,
		PermissionWriteAlerts,
	},
	contracts.RoleDeveloper: {
		PermissionViewDevices,
		PermissionViewProjects,
		PermissionViewDiscovery,
		PermissionViewK8sResources,
		PermissionViewK8sPodLogs,
		PermissionViewPipelines,
		PermissionViewServiceCatalog,
		PermissionViewPhysicalHosts,
		PermissionViewLogs,
		PermissionViewMetrics,
		PermissionViewAlerts,
	},
	contracts.RoleAuditor: {
		PermissionViewDevices,
		PermissionViewProjects,
		PermissionViewDiscovery,
		PermissionViewK8sResources,
		PermissionViewK8sPodLogs,
		PermissionViewPipelines,
		PermissionViewServiceCatalog,
		PermissionViewPhysicalHosts,
		PermissionViewAuditLog,
		PermissionViewLogs,
		PermissionViewMetrics,
		PermissionViewAlerts,
	},
	// RoleScopedAuditor has the same view-permissions as
	// RoleAuditor (read-only) PLUS the per-tenant audit-log
	// permission. Per-tenant routing in audit.Service.ListForCaller
	// (added in v0.2.0.0 scoped-Auditor follow-up) uses
	// the membership cache built into caller.Caller.
	contracts.RoleScopedAuditor: {
		PermissionViewDevices,
		PermissionViewProjects,
		PermissionViewDiscovery,
		PermissionViewK8sResources,
		PermissionViewK8sPodLogs,
		PermissionViewPipelines,
		PermissionViewServiceCatalog,
		PermissionViewPhysicalHosts,
		PermissionViewAuditLog,
		PermissionViewAuditLogProject,
		PermissionViewLogs,
		PermissionViewMetrics,
		PermissionViewAlerts,
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
