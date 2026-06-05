package device

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// JSONMap is a generic map[string]any column. We keep the type
// local to the device package because no other module currently
// needs a JSON column, and lifting it into pkg/contracts would
// force an import dependency on GORM that we do not want there.
//
// Value() and Scan() implement the database/sql.Scanner /
// driver.Valuer pair so GORM can persist the map into a TEXT or
// JSONB column depending on the driver. The JSON is sorted on
// Scan into a fresh map so callers cannot mutate the cached
// representation.
type JSONMap map[string]any

// Value renders the map as a JSON byte slice. nil maps render
// as NULL so the column stays clean for rows that have no
// labels / metadata.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any(m))
}

// Scan parses the column value into a fresh map. nil inputs
// (NULL columns) produce a nil map. The function tolerates
// empty strings by treating them as "no value" — some drivers
// return "" for NULL on certain column types.
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
		return errors.New("device: JSONMap: unsupported scan source type")
	}
	out := JSONMap{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// GormDataType pins the column type. SQLite uses TEXT; PostgreSQL
// uses JSONB. GORM inspects this when AutoMigrate creates the
// table.
func (JSONMap) GormDataType() string {
	return "text"
}

// Device is the unified device-management model. It captures the
// cross-cutting concerns shared by PhysicalHost, VM, and
// NetworkDevice. The type-specific fields (CPU, memory, vendor,
// etc.) live in the per-type packages; this struct is the row
// the device-management subsystem writes to.
type Device struct {
	database.BaseModel

	// Name is the human-readable device label. Required.
	Name string `gorm:"column:name;size:128;not null;index" json:"name"`

	// Type is one of the DeviceType constants. Required.
	Type DeviceType `gorm:"column:type;size:32;not null;index" json:"type"`

	// State is one of the DeviceState constants. Defaults to
	// "online" on insert; the service layer enforces the
	// transition rules.
	State DeviceState `gorm:"column:state;size:32;not null;index;default:online" json:"state"`

	// GroupID is a nullable FK to device_groups.id. NULL means
	// "not in a group"; a non-nil pointer is required when the
	// repository joins against the group.
	GroupID *string `gorm:"column:group_id;type:text;index" json:"group_id,omitempty"`

	// TemplateID is a nullable FK to configuration_templates.id.
	TemplateID *string `gorm:"column:template_id;type:text;index" json:"template_id,omitempty"`

	// Labels carries arbitrary key/value tags used for label-based
	// RBAC and ad-hoc search. Stored as JSON (TEXT on sqlite,
	// JSONB on postgres).
	Labels JSONMap `gorm:"column:labels;type:text" json:"labels,omitempty"`

	// Metadata carries device-type-specific extensions that do
	// not deserve their own column (firmware, BIOS version, etc.).
	Metadata JSONMap `gorm:"column:metadata;type:text" json:"metadata,omitempty"`

	// LastSeen is the timestamp of the most recent successful
	// heartbeat. Nil means "never seen" — i.e. the device has
	// been registered but has not yet reported in.
	LastSeen *time.Time `gorm:"column:last_seen;index" json:"last_seen,omitempty"`
}

// TableName pins the GORM-generated table name. Hard-coding it
// here keeps the SQL predictable for migrations and ad-hoc
// queries.
func (Device) TableName() string { return "devices" }

// BeforeCreate wires the GORM hook. We default the State to
// "online" so a service-layer caller can leave it blank, and
// explicitly call the embedded BaseModel's BeforeCreate so the
// UUID PK is assigned — GORM does not invoke promoted hooks
// from embedded structs when a top-level method of the same
// name exists.
func (d *Device) BeforeCreate(tx *gorm.DB) error {
	if d.State == "" {
		d.State = DeviceStateOnline
	}
	return d.BaseModel.BeforeCreate(tx)
}

// AllModels returns every GORM model this package owns. The
// device.AutoMigrate entry point uses this so callers do not
// have to repeat the model list every time the package grows.
func AllModels() []any {
	return []any{
		&Device{},
		&DeviceGroup{},
		&ConfigurationTemplate{},
	}
}
