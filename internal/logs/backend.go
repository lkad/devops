package logs

import (
	"context"
	"time"
)

// Capabilities describes what a LogBackend can do. The struct is
// deliberately small — the spec scenario "Interface contract" only
// asks for Query/Stats/Health/Capabilities, and the "Capabilities
// response" scenario in docs/LOG-QUERY-API.md §5.1 elaborates the
// fields that clients read. We keep a tight Go-level surface so
// adding a backend is a matter of populating the struct.
//
// SupportsAggregation distinguishes backends that can return
// aggregated results (ES, Loki) from those that can only return raw
// log lines (Local). MaxTimeRange is the rolling window the backend
// is willing to serve — Loki hardcodes 30d, Local and ES do not
// have an upper bound. MaxQueryLength is the maximum number of
// characters in the search text (used by the QueryTooLong check).
// BackendName is the wire-format identifier (e.g. "local",
// "elasticsearch", "loki").
type Capabilities struct {
	SupportsAggregation bool
	MaxTimeRange        time.Duration
	MaxQueryLength      int
	BackendName         string
}

// LogBackend is the seam every concrete storage backend implements.
// The service layer talks to this interface so swapping Local for ES
// for Loki is a one-line configuration change.
type LogBackend interface {
	// Capabilities returns a value-copy of the backend's feature
	// surface. The result must be safe to compare with == and
	// must not be mutated by the caller.
	Capabilities() Capabilities

	// Query runs a universal Query and returns a Result. The
	// service is responsible for filling defaults before the call.
	Query(ctx context.Context, q Query) (Result, error)

	// Streams enumerates the available named streams. Used by
	// the /streams endpoint and by the UI to populate source
	// dropdowns.
	Streams(ctx context.Context) ([]Stream, error)
}
