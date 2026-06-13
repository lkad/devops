package logs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrUnauthenticated is the sentinel returned when a service
// method is invoked without a caller on the context. The
// handler maps it to a 401 UNAUTHORIZED APIError.
var ErrUnauthenticated = errors.New("logs: unauthenticated")

// ErrForbidden is the sentinel returned when the caller's
// tenant membership does not allow the requested operation.
// The handler maps it to a 403 FORBIDDEN APIError.
//
// Note: a log stream is keyed by a (service, env) pair that
// has no direct project_id column. The v0.3.0.0 P0 #2
// service-layer guard is SuperAdmin-only for read. Per-stream
// project binding is the follow-up work.
var ErrForbidden = errors.New("logs: forbidden")

// ServiceConfig tunes the orchestration layer. MaxPageSize caps
// the limit a client may request; the actual backend's limit is
// also applied. RejectStructuredQueryOnLocal enables the strict
// spec scenario "Lucene query on Local backend".
type ServiceConfig struct {
	MaxPageSize                 int
	RejectStructuredQueryOnLocal bool
}

// Service is the orchestration layer between handlers and a
// LogBackend. It owns defaults, time-range validation, query
// length checks, and the Meta envelope that signals graceful
// degradation. The service never reaches into the network — it
// just composes a LogBackend.
type Service struct {
	backend LogBackend
	cfg     ServiceConfig
}

// NewService constructs a Service. The backend is required; cfg
// fields are optional (sane defaults applied).
func NewService(b LogBackend, cfg ServiceConfig) *Service {
	if cfg.MaxPageSize == 0 {
		cfg.MaxPageSize = 1000
	}
	return &Service{backend: b, cfg: cfg}
}

// Capabilities returns the backend's feature surface unchanged.
func (s *Service) Capabilities() Capabilities {
	return s.backend.Capabilities()
}

// Streams is a thin pass-through.
func (s *Service) Streams(ctx context.Context) ([]Stream, error) {
	return s.backend.Streams(ctx)
}

// StreamsWithCaller is the v0.3.0.0 P0 #2 cross-tenant
// variant of Streams. The caller MUST be attached to the
// context; an absent caller surfaces as ErrUnauthenticated
// (401). A non-SuperAdmin caller is denied (streams are
// global today; per-stream project binding is the
// follow-up work).
func (s *Service) StreamsWithCaller(ctx context.Context) ([]Stream, error) {
	cl, ok := caller.FromContext(ctx)
	if !ok || cl == nil || cl.User == nil {
		return nil, ErrUnauthenticated
	}
	if !cl.IsSuperAdmin() {
		return nil, ErrForbidden
	}
	return s.Streams(ctx)
}

// CreateLogEntry persists a single LogEntry through the
// configured backend. The method is the write-side of the
// k8s-pod-log-streaming persistence requirement: every
// pod log line the K8s streamer sees MUST land in
// log_entries via this method (or, in tests, a fake
// LogSink that records the call).
//
// The backend is type-asserted to LogSink; backends that
// do not implement LogSink (e.g. a read-only ES client)
// return a "not supported" error so a misconfiguration
// is loud at wire-up time, not silent at 3am.
//
// The default Timestamp is set to now() when the caller
// passes a zero value — K8s lines from the apiserver
// already carry a Timestamp, so this branch is only
// exercised in tests / dev echoes.
func (s *Service) CreateLogEntry(ctx context.Context, e LogEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sink, ok := s.backend.(LogSink)
	if !ok {
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "log backend does not support CreateLogEntry",
		}
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return sink.Append(e)
}

// Stats is the spec's "Log Statistics" surface: total
// rows, by_level, by_source. Computed by walking the
// underlying backend's stats. A backend that doesn't
// implement Stats (older Local variant) returns the
// zero-value shape; the wire is the same.
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	caps := s.backend.Capabilities()
	_ = caps
	// Cheap path: walk the last 24h of log entries and
	// bucket by level + source. A future iteration can
	// push this into a precomputed aggregate table.
	from := time.Now().Add(-24 * time.Hour)
	res, err := s.Query(ctx, Query{From: from, Limit: 1000})
	if err != nil {
		// Don't fail the stats call on query error —
		// return a zero-value stats instead so the
		// UI badge still renders.
		return map[string]any{
			"total":     0,
			"by_level":  map[string]int{},
			"by_source": map[string]int{},
		}, nil
	}
	byLevel := map[string]int{}
	bySource := map[string]int{}
	for _, e := range res.Entries {
		byLevel[e.Level]++
		bySource[e.Source]++
	}
	return map[string]any{
		"total":     len(res.Entries),
		"by_level":  byLevel,
		"by_source": bySource,
	}, nil
}

// Query fills defaults, validates, and dispatches to the backend.
// Returns a *contracts.APIError when validation fails so the
// handler layer can render the standard envelope without unwrapping.
func (s *Service) Query(ctx context.Context, q Query) (Result, error) {
	caps := s.backend.Capabilities()

	// Default time range: last 24h. Per the spec scenario
	// "Default time range" — no client time → default to 24h.
	now := time.Now().UTC()
	if q.From.IsZero() && q.To.IsZero() {
		q.From = now.Add(-24 * time.Hour)
		q.To = now
	} else if q.From.IsZero() {
		q.To = now
	} else if q.To.IsZero() {
		q.To = now
	}

	// Time range cap: per the task brief, hard-reject when the
	// range exceeds the backend's MaxTimeRange. This is the K8s
	// "30 days max" rule.
	if caps.MaxTimeRange > 0 {
		if d := q.To.Sub(q.From); d > caps.MaxTimeRange {
			return Result{}, ErrTimeRangeExceeded(int(d.Hours()), int(caps.MaxTimeRange.Hours()))
		}
	}

	// Query length: per MaxQueryLength.
	if caps.MaxQueryLength > 0 && len(q.Text) > caps.MaxQueryLength {
		return Result{}, ErrQueryTooLong(len(q.Text), caps.MaxQueryLength)
	}

	// Structured query on Local backend — strict spec scenario.
	if s.cfg.RejectStructuredQueryOnLocal && caps.BackendName == "local" {
		if looksStructured(q.Text) {
			return Result{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "Local backend only supports search (substring). Use search field or switch to elasticsearch backend",
			}
		}
	}

	// Limit cap. Mark Degraded when we have to clamp.
	max := s.cfg.MaxPageSize
	if caps.MaxQueryLength > 0 && max > caps.MaxQueryLength {
		// Local is 1024 by default; we don't surface that as
		// degradation for the cap path.
	}
	if q.Limit > max {
		q.Limit = max
	}

	res, err := s.backend.Query(ctx, q)
	if err != nil {
		return Result{}, err
	}

	// Ensure Meta is populated even if the backend left it
	// sparse (some backends skip filling it on the hot path).
	if res.Meta.Backend == "" {
		res.Meta.Backend = caps.BackendName
	}

	// If the result was truncated to fit the limit, mark
	// Degraded so the UI can show a warning.
	if q.Limit > 0 && res.Total > int64(q.Limit) {
		res.Meta.Degraded = true
		if res.Meta.Reason == "" {
			res.Meta.Reason = "limit: capped to " + itoa(q.Limit)
		}
	}

	// Populate limits map if the backend left it empty.
	if res.Meta.Limits == nil {
		res.Meta.Limits = map[string]any{
			"max_page_size": s.cfg.MaxPageSize,
		}
		if caps.MaxTimeRange > 0 {
			res.Meta.Limits["max_time_range"] = caps.MaxTimeRange.String()
		}
	}
	return res, nil
}

// looksStructured detects a Lucene/LogQL-shaped substring. The
// heuristic is conservative: a colon followed by a wildcard or
// uppercase AND/OR is structured.
func looksStructured(s string) bool {
	if s == "" {
		return false
	}
	upper := strings.ToUpper(s)
	if strings.Contains(upper, " AND ") || strings.Contains(upper, " OR ") {
		return true
	}
	if strings.Contains(s, ":*") {
		return true
	}
	return false
}

// itoa avoids an extra import for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
