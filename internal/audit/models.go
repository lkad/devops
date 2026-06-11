// Package audit implements the audit-logging subsystem. It owns
// the AuditEvent row, the read-side repository, the emitter seam
// (DBEmitter / BufferedEmitter / NoopEmitter), the query service,
// the two HTTP handlers, and the request-scoped middleware that
// surfaces the request actor / IP / user-agent to other modules'
// service-layer RecordAction calls.
//
// Layered rules (per the implementation playbook):
//
//	handler -> service -> emitter / repository -> model
//
// The Emitter is its own type (not just a method on the
// repository) so a unit test can swap in a NoopEmitter or a
// recording emitter without touching GORM. Production wires
// DBEmitter (which writes via the repository) wrapped in a
// BufferedEmitter (which drops on overflow with a warning).
//
// The module is append-only at the API surface: there is no
// update or delete endpoint. Maintenance windows and retention
// policies are out of scope for the API (a separate retention
// job is planned).
package audit

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// AuditAction is the verb of an audit event. Values are
// "<resource>.<action>" so the column is greppable across modules
// and the spec's "action" field can be filtered directly. The
// constants below are the canonical set; the spec also allows
// ad-hoc values for new modules.
type AuditAction string

const (
	ActionCreate         AuditAction = "create"
	ActionUpdate         AuditAction = "update"
	ActionDelete         AuditAction = "delete"
	ActionMaintenanceEnter AuditAction = "maintenance_enter"
	ActionMaintenanceExit  AuditAction = "maintenance_exit"
	ActionAcknowledge    AuditAction = "acknowledge"
	ActionResolve        AuditAction = "resolve"
	ActionMemberAdd      AuditAction = "member_add"
	ActionMemberRemove   AuditAction = "member_remove"
	ActionTrigger        AuditAction = "trigger"
	ActionCancel         AuditAction = "cancel"
)

// AuditResourceType is the noun of an audit event. Mirrors the
// module ownership; matches the value in the existing models
// (project, device, physicalhost, alert, k8s_cluster, pipeline).
type AuditResourceType string

const (
	ResourceBusinessLine  AuditResourceType = "business_line"
	ResourceSystem        AuditResourceType = "system"
	ResourceProject       AuditResourceType = "project"
	ResourceResourceLink  AuditResourceType = "resource_link"
	ResourcePermission    AuditResourceType = "permission"
	ResourceDevice        AuditResourceType = "device"
	ResourcePhysicalHost  AuditResourceType = "physicalhost"
	ResourceAlert         AuditResourceType = "alert"
	ResourceK8sCluster    AuditResourceType = "k8s_cluster"
	ResourcePipeline      AuditResourceType = "pipeline"
	ResourceProjectMember AuditResourceType = "project_member"
	// SavedFilter / AlertRule live in the logs module but
	// share the audit table; a saved filter and an alert rule
	// are independent nouns so each gets its own resource
	// type rather than collapsing into a generic "log_config".
	ResourceSavedFilter   AuditResourceType = "log_saved_filter"
	ResourceAlertRule     AuditResourceType = "log_alert_rule"
	// OnCall / Runbook are sub-resources of a Service in
	// the service-catalog module. The audit resource type
	// identifies the sub-resource so the audit list view can
	// filter by it; the parent service_id is captured in
	// Metadata.
	ResourceOnCall        AuditResourceType = "service_oncall"
	ResourceRunbook       AuditResourceType = "service_runbook"
	// DiscoveryRun / DiscoveryHost cover the network-discovery
	// module. PromoteHosts emits two events (one for the run,
	// one per host promoted) so a future "what was discovered"
	// view can correlate.
	ResourceDiscoveryRun  AuditResourceType = "discovery_run"
	ResourceDiscoveryHost AuditResourceType = "discovery_host"
)

// JSONMap is the schema-less key/value bag used by the Metadata
// column. It works on both Postgres (jsonb) and SQLite (TEXT)
// without the caller having to know the driver. The zero value is
// a usable empty map; a nil map is encoded as "{}" so the column
// never has to deal with NULL vs empty-object ambiguity.
type JSONMap map[string]any

// Value renders the map to JSON for the database driver.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(m))
}

// Scan parses a database value into the map. The destination is
// left as a non-nil empty map on a NULL / empty input so callers
// never have to nil-check before ranging.
func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = JSONMap{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		if v == "" {
			*m = JSONMap{}
			return nil
		}
		data = []byte(v)
	default:
		return fmt.Errorf("audit.JSONMap.Scan: unsupported src type %T", src)
	}
	if len(data) == 0 {
		*m = JSONMap{}
		return nil
	}
	var out JSONMap
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("audit.JSONMap.Scan: %w (payload=%s)", err, string(data))
	}
	if out == nil {
		out = JSONMap{}
	}
	*m = out
	return nil
}

// GormDataType hints GORM to use TEXT/jsonb appropriately. GORM
// v2 picks the right column type for the active driver (jsonb on
// Postgres, TEXT on SQLite).
func (JSONMap) GormDataType() string { return "json" }

// Scanner is the subset of sql.Scanner this package depends on.
// Declared locally so the package does not need to import
// database/sql in its public surface area.
type Scanner interface {
	Scan(src any) error
}

// AuditEvent is the row the audit-logging subsystem writes. One
// row per state-changing operation. Append-only at the API
// surface; soft-deletes are not exposed.
//
// Field naming follows the wire spec ("ip_address",
// "user_agent", "actor_id", "actor_username") so the JSON the
// frontend renders matches the spec verbatim. The Metadata
// column is a free-form bag for "before / after" snapshots or
// any other per-action context the calling module wants to keep.
type AuditEvent struct {
	database.BaseModel

	Action        AuditAction       `gorm:"column:action;size:64;not null;index" json:"action"`
	ActorID       string            `gorm:"column:actor_id;type:text;index" json:"actor_id"`
	ActorUsername string            `gorm:"column:actor_username;type:text;index" json:"actor_username"`
	ResourceType  AuditResourceType `gorm:"column:resource_type;size:64;not null;index" json:"resource_type"`
	ResourceID    string            `gorm:"column:resource_id;type:text;index" json:"resource_id"`
	Metadata      JSONMap           `gorm:"column:metadata;type:text" json:"metadata,omitempty"`
	IPAddress     string            `gorm:"column:ip_address;type:text" json:"ip_address,omitempty"`
	UserAgent     string            `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	OccurredAt    time.Time         `gorm:"column:occurred_at;not null;index" json:"occurred_at"`
}

// TableName pins the GORM-generated table name. Hard-coding
// keeps the SQL predictable for migrations and ad-hoc queries;
// the spec calls for the historical "audit_logs" table.
func (AuditEvent) TableName() string { return "audit_logs" }

// BeforeCreate wires the GORM hook. We delegate to the embedded
// BaseModel's BeforeCreate so the UUID PK is assigned, but we do
// NOT default OccurredAt here — the service layer owns the
// timestamp so the clock stays a single source of truth.
func (e *AuditEvent) BeforeCreate(tx *gorm.DB) error {
	return e.BaseModel.BeforeCreate(tx)
}

// AllModels returns every GORM model this package owns. The
// registration point in cmd/devops-toolkit/main.go uses this so
// the model list does not have to be repeated.
func AllModels() []any {
	return []any{
		&AuditEvent{},
	}
}

// ensure compile-time interfaces.
var (
	_ driver.Valuer = JSONMap{}
	_ Scanner       = (*JSONMap)(nil)
)
