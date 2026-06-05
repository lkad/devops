package alerts

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONMap is a generic map[string]any field that GORM can persist
// transparently. The same shape is used by every other module
// (device, project) but the project rules forbid an
// internal/contracts dependency from a leaf module, so we declare
// it locally to keep the alert package self-contained.
//
// The zero value is a usable empty map. Scan accepts nil
// destinations and empty strings as "no value" so the column
// stays optional. Value encodes a nil map as "{}" so the column
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
		return fmt.Errorf("alerts.JSONMap.Scan: unsupported src type %T", src)
	}
	if len(data) == 0 {
		*m = JSONMap{}
		return nil
	}
	var out JSONMap
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("alerts.JSONMap.Scan: %w (payload=%s)", err, string(data))
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

// StringList is a slice of strings persisted as a JSON array. The
// alert-rules table needs an FK-list without the overhead of a
// join table for the small N (<= 10) the rule model allows.
type StringList []string

// Value renders the slice as a JSON array.
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(s))
}

// Scan parses a database value into the slice. A NULL / empty
// input produces an empty (non-nil) slice.
func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = StringList{}
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		if v == "" {
			*s = StringList{}
			return nil
		}
		data = []byte(v)
	default:
		return fmt.Errorf("alerts.StringList.Scan: unsupported src type %T", src)
	}
	if len(data) == 0 {
		*s = StringList{}
		return nil
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("alerts.StringList.Scan: %w (payload=%s)", err, string(data))
	}
	*s = out
	return nil
}

// GormDataType pins the column type to TEXT.
func (StringList) GormDataType() string { return "text" }

// ensure interfaces at compile time.
var (
	_ driver.Valuer = JSONMap{}
	_ Scanner       = (*JSONMap)(nil)
	_ driver.Valuer = StringList{}
	_ Scanner       = (*StringList)(nil)
)

// Scanner is the subset of sql.Scanner the package depends on. It
// lets us assert JSONMap / StringList satisfy the contract
// without pulling database/sql into the public surface area.
type Scanner interface {
	Scan(src any) error
}

// errNilReceiver is reserved for future nil-receiver guards. The
// current Scan tolerates a nil pointer (it writes through) so
// the var is not used today.
var errNilReceiver = errors.New("alerts: nil receiver")
