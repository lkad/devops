// Package logs implements the log-aggregation subsystem per
// openspec/specs/log-aggregation/spec.md and docs/LOG-QUERY-API.md.
//
// The package is laid out as handler → service → backend. The
// LogBackend interface is the seam — Local is wired in tests; ES and
// Loki are wired in production behind a configurable HTTP client so
// tests never reach the network.
package logs

import "time"

// LogEntry is the universal log line shape returned by every
// backend. The JSON tag set is frozen for v1 — see the spec
// scenario "LogEntry schema frozen".
type LogEntry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Source    string            `json:"source"`
	Message   string            `json:"message"`
	Host      string            `json:"host,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Fields    map[string]any    `json:"fields,omitempty"`
}

// Filter is a key/op/value predicate. Ops are deliberately small so
// the Local backend can implement them in-memory; ES and Loki can
// translate the same shape to Lucene / LogQL.
type Filter struct {
	Key   string `json:"key"`
	Op    string `json:"op"` // eq | neq | prefix | suffix | contains | regex
	Value string `json:"value"`
}

// Query is the universal query DSL accepted by every backend. All
// fields are optional; the service layer fills in defaults (24h time
// range, 50 limit) before calling a backend.
type Query struct {
	Text    string    `json:"q,omitempty"`
	From    time.Time `json:"from,omitempty"`
	To      time.Time `json:"to,omitempty"`
	Filters []Filter  `json:"filters,omitempty"`
	Limit   int       `json:"limit,omitempty"`
	Sort    string    `json:"sort,omitempty"` // "time:asc" | "time:desc"
}

// Stream is one named source of log lines (a stream label set in
// Loki, a host in the Local backend, etc.).
type Stream struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Result is the universal response envelope. Meta is non-nil even on
// success so the spec's "Response Meta Envelope" scenario is
// satisfied without an extra nil check.
type Result struct {
	Entries []LogEntry `json:"entries"`
	Total   int64      `json:"total"`
	Meta    Meta       `json:"meta"`
}
