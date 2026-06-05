package discovery

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// JSONMap is the local GORM-friendly map[string]any column
// used for OpenPorts and any other bag-of-attributes the
// discovery subsystem needs to persist. We keep it local for
// the same reason the device package keeps its own: lifting it
// into pkg/contracts would force a GORM dependency on the
// shared types, which is the wrong direction.
type JSONMap map[string]any

// Value renders the map as a JSON byte slice. nil maps render
// as NULL so empty rows stay clean.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any(m))
}

// Scan parses the column value into a fresh map.
func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		if v == "" {
			*m = nil
			return nil
		}
		b = []byte(v)
	default:
		return errors.New("discovery: JSONMap: unsupported scan source type")
	}
	out := JSONMap{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// GormDataType pins the column type. SQLite uses TEXT; the
// same declaration renders as JSONB on PostgreSQL via the
// per-driver override GORM applies during migration.
func (JSONMap) GormDataType() string {
	return "text"
}

// RunStatus is the lifecycle of a discovery run. The four
// states match the spec's "scan status" requirement: a run is
// created in "running" state and transitions to "completed"
// (or "failed") when the scan finishes.
type RunStatus string

const (
	// RunStatusRunning is the initial state. The service
	// transitions out of it once Scanner.Scan returns.
	RunStatusRunning RunStatus = "running"
	// RunStatusCompleted means the scan finished and at
	// least one host was probed successfully.
	RunStatusCompleted RunStatus = "completed"
	// RunStatusFailed means the scan aborted (network down,
	// cancellation, or scanner-level error).
	RunStatusFailed RunStatus = "failed"
	// RunStatusEmpty means the scan finished but found no
	// hosts. It is distinct from "failed" — the network was
	// reachable, it just had no live hosts in range.
	RunStatusEmpty RunStatus = "empty"
)

// Valid reports whether s is one of the recognised run states.
func (s RunStatus) Valid() bool {
	switch s {
	case RunStatusRunning, RunStatusCompleted, RunStatusFailed, RunStatusEmpty:
		return true
	}
	return false
}

// DiscoveryRun is the parent record for a single scan. It
// captures the parameters (CIDR), timing (StartedAt /
// CompletedAt), outcome (Status, HostsFound), and the
// standard GORM soft-delete / timestamp columns inherited
// from database.BaseModel.
type DiscoveryRun struct {
	database.BaseModel

	// CIDR is the network range that was scanned. Required.
	CIDR string `gorm:"column:cidr;size:64;not null;index" json:"cidr"`

	// StartedAt is the wall-clock time the scan began.
	StartedAt time.Time `gorm:"column:started_at;not null;index" json:"started_at"`

	// CompletedAt is nil while the run is in flight. The
	// service sets it when the scanner returns.
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`

	// HostsFound is the count of distinct live hosts the
	// scan discovered. Populated when the run completes.
	HostsFound int `gorm:"column:hosts_found;not null;default:0" json:"hosts_found"`

	// Status is one of the RunStatus constants.
	Status RunStatus `gorm:"column:status;size:32;not null;index;default:running" json:"status"`
}

// TableName pins the table name. Hard-coding keeps SQL
// predictable for migrations and ad-hoc queries.
func (DiscoveryRun) TableName() string { return "discovery_runs" }

// BeforeCreate wires the GORM hook. We default the Status to
// "running" so a service-layer caller can leave it blank, and
// we explicitly invoke the embedded BaseModel's BeforeCreate
// so the UUID PK is assigned — GORM does not invoke promoted
// hooks from embedded structs when a top-level method of the
// same name exists.
func (d *DiscoveryRun) BeforeCreate(tx *gorm.DB) error {
	if d.Status == "" {
		d.Status = RunStatusRunning
	}
	return d.BaseModel.BeforeCreate(tx)
}

// DiscoveredHost is the per-host output of a single scan. It
// is the row that "promote to Device" mutates: when a user
// promotes a host, the service creates a Device and writes the
// resulting device ID back into PromotedToDeviceID so a
// second promotion is a no-op.
type DiscoveredHost struct {
	database.BaseModel

	// RunID is the parent DiscoveryRun. The cascade-delete
	// behaviour is application-level: deleting a run
	// soft-deletes its hosts via the repository's DeleteRun
	// method.
	RunID string `gorm:"column:run_id;type:text;not null;index" json:"run_id"`

	// IPAddress is the canonical key for dedup. The spec
	// says "Result deduplication across scans" — we enforce
	// (run_id, ip_address) as the dedup key, but a single
	// run can only see a given IP once.
	IPAddress string `gorm:"column:ip_address;size:64;not null;index:idx_host_run_ip,priority:2" json:"ip_address"`

	// Hostname is the reverse-DNS result. Empty when the
	// PTR record is missing.
	Hostname string `gorm:"column:hostname;size:255" json:"hostname,omitempty"`

	// OpenPorts is the per-host port list. Stored as JSON so
	// a future schema can grow without a migration.
	OpenPorts JSONMap `gorm:"column:open_ports;type:text" json:"open_ports,omitempty"`

	// SNMPSysDescr is the sysDescr.0 MIB value, or "" when
	// no SNMP response was received.
	SNMPSysDescr string `gorm:"column:snmp_sys_descr;size:512" json:"snmp_sys_descr,omitempty"`

	// PromotedToDeviceID is the FK into the devices table
	// once the user promotes the host. NULL means "still a
	// discovered host, not yet a device".
	PromotedToDeviceID *string `gorm:"column:promoted_to_device_id;type:text;index" json:"promoted_to_device_id,omitempty"`
}

// TableName pins the table name.
func (DiscoveredHost) TableName() string { return "discovered_hosts" }

// AllModels returns every GORM model this package owns. The
// AutoMigrate entry point (and the test fixture) use this so
// callers do not have to repeat the model list every time the
// package grows.
func AllModels() []any {
	return []any{
		&DiscoveryRun{},
		&DiscoveredHost{},
	}
}
