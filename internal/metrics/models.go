// Package metrics implements the metrics-collection subsystem. It
// collects and exposes time-series metrics (cpu, mem, disk, net)
// that are scraped from physical hosts (via the physicalhost
// package) and Kubernetes pods (via the k8s package). The model
// is intentionally generic — anything that emits a labelled
// float-valued time-series can be stored here.
//
// Layered rules (per the implementation playbook):
//
//	handler -> service -> repository -> model
//
// The Scraper is its own type (defined in scraper.go) so the
// service can ingest metrics without taking a direct dependency
// on the network. Tests use the Fake scraper; production uses
// the PrometheusScraper that pulls a Prometheus /metrics
// endpoint.
package metrics

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/devops-toolkit/backend/internal/database"
)

// TargetType identifies the kind of entity a Metric is
// attached to. The spec recognises physical hosts and K8s pods;
// new types can be added without a schema change because the
// column is plain text.
type TargetType string

const (
	// TargetPhysicalHost covers metrics scraped from
	// /metrics endpoints exported by node-exporter-style
	// agents running on a physical host.
	TargetPhysicalHost TargetType = "physical_host"
	// TargetK8sPod covers metrics scraped from pod-local
	// exporters (kube-state-metrics is handled separately).
	TargetK8sPod TargetType = "k8s_pod"
)

// Valid reports whether t is one of the recognised target
// types. Unknown values are rejected by the service layer with
// a 400 VALIDATION_ERROR.
func (t TargetType) Valid() bool {
	switch t {
	case TargetPhysicalHost, TargetK8sPod:
		return true
	}
	return false
}

// MetricName is the conventional name of a time-series
// (cpu, mem, disk, net, ...). The service layer treats this as
// free text — there is no fixed enum — so adding a new metric
// does not require a code change.
type MetricName string

const (
	NameCPU MetricName = "cpu"
	NameMem MetricName = "mem"
	NameDisk MetricName = "disk"
	NameNet MetricName = "net"
)

// JSONMap is a generic map[string]any column. Mirrors the
// type used by the device package — kept local so the metrics
// package does not import from device. The column stores
// labels in JSON (TEXT on sqlite, JSONB on postgres).
type JSONMap map[string]any

// Value renders the map as a JSON byte slice. nil maps render
// as NULL so the column stays clean for rows that have no
// labels.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any(m))
}

// Scan parses the column value into a fresh map. nil inputs
// (NULL columns) produce a nil map. Empty strings are
// tolerated as "no value" — some drivers return "" for NULL
// on certain column types.
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
		return errors.New("metrics: JSONMap: unsupported scan source type")
	}
	out := JSONMap{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// GormDataType pins the column type. SQLite uses TEXT;
// PostgreSQL uses JSONB. GORM inspects this when AutoMigrate
// creates the table.
func (JSONMap) GormDataType() string {
	return "text"
}

// Metric is the row the metrics subsystem writes to. One row
// is one observation of one time-series. Time-series grouping
// happens at query time via MetricSeries (Name + TargetType +
// TargetID).
type Metric struct {
	database.BaseModel

	// Name is the metric identifier (cpu, mem, disk, net, ...).
	// Indexed for fast series lookups.
	Name string `gorm:"column:name;size:64;not null;index:idx_metrics_series" json:"name"`

	// TargetType is the kind of entity the metric is attached
	// to. Indexed so the series-listing query can group by
	// (target_type, target_id) cheaply.
	TargetType string `gorm:"column:target_type;size:32;not null;index:idx_metrics_series" json:"target_type"`

	// TargetID is the device_id / pod name the metric is
	// attached to. Indexed for the same reason.
	TargetID string `gorm:"column:target_id;size:128;not null;index:idx_metrics_series" json:"target_id"`

	// Value is the observation. Plain float64 — counters and
	// gauges are both represented; the spec does not
	// distinguish between them at the storage layer.
	Value float64 `gorm:"column:value;not null" json:"value"`

	// Timestamp is the time the observation was recorded at
	// the source. The BaseModel.CreatedAt is when the row
	// reached our database, which is close but not identical
	// (network/scrape latency). Indexed for time-range scans.
	Timestamp time.Time `gorm:"column:timestamp;not null;index" json:"timestamp"`

	// Labels carries arbitrary key/value tags (region,
	// cluster, instance, ...). Stored as JSON.
	Labels JSONMap `gorm:"column:labels;type:text" json:"labels,omitempty"`
}

// TableName pins the GORM-generated table name. Hard-coding
// it here keeps the SQL predictable for migrations and ad-hoc
// queries.
func (Metric) TableName() string { return "metrics" }

// AllModels returns every GORM model this package owns. The
// registration point in cmd/devops-toolkit/main.go uses this
// so the model list does not have to be repeated.
func AllModels() []any {
	return []any{
		&Metric{},
	}
}

// MetricSeries is the in-memory grouping of a time-series.
// The wire format for GET /api/v1/metrics/series/:name is a
// list of MetricSeries; each one carries the metadata of the
// series plus the data points.
type MetricSeries struct {
	// Name is the metric name (cpu, mem, ...).
	Name string `json:"name"`
	// TargetType is the kind of entity the metric is
	// attached to.
	TargetType string `json:"target_type"`
	// TargetID is the device_id / pod name.
	TargetID string `json:"target_id"`
	// Labels is the labelset that distinguishes this series
	// from sibling series of the same (name, target).
	Labels JSONMap `json:"labels,omitempty"`
	// Points are the observations ordered by timestamp
	// ascending. Value is the observed float; Timestamp is
	// when the source recorded it.
	Points []DataPoint `json:"points"`
}

// DataPoint is a single observation in a MetricSeries. The
// (Timestamp, Value) pair is what the dashboard plots.
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MaxQueryRange is the maximum (to - from) duration the
// service will accept on a list / series call. Longer ranges
// are rejected with 422 INVALID_STATE so the caller has to
// narrow the window. 90 days matches the device package's
// long-term storage horizon and is the documented choice
// (per the spec).
const MaxQueryRange = 90 * 24 * time.Hour

// IngestInput is the request payload for Service.Ingest.
// The handler decodes the wire JSON into this struct; the
// service then validates and maps it onto a Metric.
type IngestInput struct {
	Name       string
	TargetType string
	TargetID   string
	Value      float64
	Timestamp  time.Time
	Labels     JSONMap
}

// ListFilter narrows the result of a Repository.List call.
// All fields are optional; the zero value returns every row.
type ListFilter struct {
	// Name filters by metric name (exact match).
	Name string
	// TargetType filters by target type (exact match).
	TargetType string
	// TargetID filters by target id (exact match).
	TargetID string
	// From is the inclusive lower bound on Timestamp. The
	// zero value means "no lower bound".
	From time.Time
	// To is the exclusive upper bound on Timestamp. The
	// zero value means "no upper bound".
	To time.Time
	// Limit caps the page size; 0 falls back to the default
	// (20, per contracts.defaultPageSize).
	Limit int
	// Offset is the number of rows to skip.
	Offset int
}
