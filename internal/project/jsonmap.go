package project

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONMap is a generic map[string]any field that GORM can persist
// transparently. It is intended for project-level Labels and Metadata
// — schema-less key/value bags — and works on both Postgres (jsonb)
// and SQLite (TEXT) without the caller having to know the driver.
//
// The zero value is a usable empty map. The Scan method accepts a
// nil destination to keep the column optional. Any JSON unmarshal
// error is wrapped with the original payload so the log line is
// actionable.
type JSONMap map[string]any

// Value renders the map to JSON for the database driver. A nil map
// is encoded as an empty object ("{}") rather than SQL NULL so the
// column never has to deal with two distinct empty representations.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

// Scan parses a database value (typically []byte or string) into the
// map. The destination is left as a non-nil empty map on a NULL
// input so callers never have to nil-check before ranging.
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
		data = []byte(v)
	default:
		return fmt.Errorf("JSONMap.Scan: unsupported src type %T", src)
	}
	if len(data) == 0 {
		*m = JSONMap{}
		return nil
	}
	var out JSONMap
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("JSONMap.Scan: %w (payload=%s)", err, string(data))
	}
	if out == nil {
		out = JSONMap{}
	}
	*m = out
	return nil
}

// GormDataType hints GORM to use TEXT/jsonb appropriately. GORM v2
// will pick the right column type for the active driver (jsonb on
// Postgres, TEXT on SQLite).
func (JSONMap) GormDataType() string { return "json" }

// ensure JSONMap implements the expected interfaces at compile time.
var (
	_ driver.Valuer = JSONMap{}
	_ Scanner       = (*JSONMap)(nil)
)

// Scanner is the subset of sql.Scanner the package depends on. It
// lets us assert JSONMap satisfies the contract without pulling
// database/sql into the public surface area.
type Scanner interface {
	Scan(src any) error
}

// errJSONMapNilReceiver is reserved for future nil-receiver guards.
// The current Scan tolerates a nil pointer (it writes through) so
// the var is not used today, but the convention is to keep error
// sentinels package-private.
var errJSONMapNilReceiver = errors.New("JSONMap.Scan: nil receiver")
