// Package physicalhost implements the physical-host monitoring subsystem.
// See physical-host-monitoring/spec.md for the authoritative requirements.
//
// Layered rules (per the implementation playbook):
//
//	handler -> service -> repository -> model
//
// The Prober is its own type so unit tests can substitute the Fake
// implementation; the real SSH client (Phase 5/6) lives in a sibling
// file and is never imported from tests.
//
// The model lives in a separate `physical_hosts` table even though
// the device-management subsystem has its own `devices` table. The
// physical-host-specific columns (IP, SSH port, maintenance fields)
// do not belong on the generic devices table; the FK from
// physical_hosts.device_id to devices.id keeps the two in sync.
package physicalhost

import (
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// PhysicalHostState is the operational lifecycle state of a
// physical host. The four values are mandated by the
// physical-host-monitoring spec and the implementation playbook
// (online / monitoring_issue / offline / maintenance).
//
// Maintenance is encoded in two places:
//
//  1. The State field — set to StateMaintenance for UI display.
//  2. The MaintenanceStartedAt field — a non-NULL pointer flags
//     the alert manager to suppress external notifications.
//
// The two are kept in sync by the MaintenanceService. A host that
// is "online AND in maintenance" is therefore represented as
// State=StateMaintenance and MaintenanceStartedAt=non-nil; the
// spec's "orthogonal" wording is honoured by keeping the
// maintenance fields as dedicated columns instead of a single
// enum.
type PhysicalHostState string

const (
	StateOnline          PhysicalHostState = "online"
	StateMonitoringIssue PhysicalHostState = "monitoring_issue"
	StateOffline         PhysicalHostState = "offline"
	StateMaintenance     PhysicalHostState = "maintenance"
)

// Valid reports whether s is one of the four recognised states.
// Used by the service layer to reject bogus create / update
// requests with a 400 VALIDATION_ERROR.
func (s PhysicalHostState) Valid() bool {
	switch s {
	case StateOnline, StateMonitoringIssue, StateOffline, StateMaintenance:
		return true
	}
	return false
}

// PhysicalHost is the row the physical-host subsystem writes to.
// It is a sibling of device.Device (1:1 via device_id) and lives
// in its own table so the physical-host-specific columns
// (IP/SSH/maintenance) do not leak into the generic devices
// schema.
type PhysicalHost struct {
	database.BaseModel

	// DeviceID is the FK to the corresponding device row. A
	// device of type physical_host should have exactly one
	// matching physical_hosts row; the uniqueIndex enforces
	// the 1:1 contract at the DB level.
	DeviceID string `gorm:"column:device_id;type:text;not null;uniqueIndex" json:"device_id"`

	// IPAddress is the management IP of the host. Required.
	IPAddress string `gorm:"column:ip_address;type:text;not null" json:"ip_address"`

	// SSHPort is the TCP port the SSH daemon listens on. Default
	// 22 is applied by the service layer when the request
	// omits the field; the column default mirrors that for
	// raw-insert callers.
	SSHPort int `gorm:"column:ssh_port;not null;default:22" json:"ssh_port"`

	// SSHUser is the account the monitor logs in as. Required.
	SSHUser string `gorm:"column:ssh_user;type:text;not null" json:"ssh_user"`

	// State is the operational state (4-state model). The
	// service layer transitions this; the column default of
	// "online" keeps raw-insert callers safe.
	State PhysicalHostState `gorm:"column:state;type:text;not null;default:online;index" json:"state"`

	// LastCheckAt is the timestamp of the most recent probe
	// attempt. Nil means "never probed" — useful for
	// distinguishing "newly registered" from "recently offline".
	LastCheckAt *time.Time `gorm:"column:last_check_at;index" json:"last_check_at,omitempty"`

	// NextCheckAt is the scheduled time of the next probe.
	// The monitor loop queries WHERE next_check_at <= now()
	// to find due hosts.
	NextCheckAt *time.Time `gorm:"column:next_check_at;index" json:"next_check_at,omitempty"`

	// ConsecutiveFails is the running count of failed probes
	// since the last successful probe. The monitor uses it to
	// implement the "drop to offline after N consecutive
	// failures" rule from the spec. Reset to 0 on any
	// successful probe.
	ConsecutiveFails int `gorm:"column:consecutive_fails;not null;default:0" json:"consecutive_fails"`

	// MaintenanceStartedAt is the start of the current
	// maintenance window. Nil means "not in maintenance".
	// Non-nil is the suppression flag the alert manager reads.
	MaintenanceStartedAt *time.Time `gorm:"column:maintenance_started_at" json:"maintenance_started_at,omitempty"`

	// MaintenanceReason is a free-text note captured at enter
	// time. The frontend renders it on the maintenance badge.
	MaintenanceReason string `gorm:"column:maintenance_reason;type:text" json:"maintenance_reason,omitempty"`

	// MaintenanceSetBy is the LDAP user who entered maintenance.
	// Captured so the audit event and the badge can both show
	// who initiated the action.
	MaintenanceSetBy string `gorm:"column:maintenance_set_by;type:text" json:"maintenance_set_by,omitempty"`
}

// TableName pins the GORM-generated table name. Hard-coding
// keeps the SQL predictable for migrations and ad-hoc queries.
func (PhysicalHost) TableName() string { return "physical_hosts" }

// BeforeCreate wires the GORM hook. We default State to online
// so service-layer callers can leave it blank, and explicitly
// call the embedded BaseModel's BeforeCreate so the UUID PK is
// assigned — GORM does not invoke promoted hooks from embedded
// structs when a top-level method of the same name exists.
func (p *PhysicalHost) BeforeCreate(tx *gorm.DB) error {
	if p.State == "" {
		p.State = StateOnline
	}
	return p.BaseModel.BeforeCreate(tx)
}

// InMaintenance is the predicate the alert manager and the
// monitor loop both call to decide whether to suppress / block
// state transitions. It is intentionally derived from the
// MaintenanceStartedAt column (not the State column) so the
// "orthogonal flag" semantics in the spec are honoured.
func (p *PhysicalHost) InMaintenance() bool {
	return p.MaintenanceStartedAt != nil
}

// AllModels returns every GORM model this package owns. The
// registration point in cmd/devops-toolkit/main.go uses this so
// the model list does not have to be repeated.
func AllModels() []any {
	return []any{
		&PhysicalHost{},
	}
}
