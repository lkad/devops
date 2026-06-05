package logs

// Meta is the response envelope the service attaches to every
// successful log query. It mirrors docs/LOG-QUERY-API.md §4.1 — the
// shape is the user-visible signal for graceful degradation.
//
// Backend is always populated; Degraded is true when the query was
// not fully satisfied (limit capped, time-range narrowed, etc.) and
// Reason carries a short human-readable explanation. Limits is a
// free-form backend-specific limits map (MaxPageSize, MaxTimeRange,
// …) so clients can render dynamic limits in the UI.
type Meta struct {
	Backend  string         `json:"backend"`
	Degraded bool           `json:"degraded"`
	Reason   string         `json:"reason,omitempty"`
	Limits   map[string]any `json:"limits,omitempty"`
}
