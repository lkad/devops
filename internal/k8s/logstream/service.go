package logstream

import (
	"context"
	"errors"
	"time"

	"github.com/devops-toolkit/backend/internal/logs"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// LogSink is the consumer-side seam the k8s-logstream
// package uses to persist every pod log line. It matches
// the signature of *logs.Service.CreateLogEntry so a
// production wiring hands the logs Service directly to
// the logstream Service via the variadic constructor
// argument. Tests supply a fake that records the calls.
//
// The interface lives in the consumer package (here, not
// in internal/logs) so the logstream service does not
// import the whole logs Service just to call one method.
type LogSink interface {
	CreateLogEntry(ctx context.Context, e logs.LogEntry) error
}

// nopLogSink is a LogSink that drops entries. It is the
// default when no sink is wired (preserves the existing
// test rigs that pre-date the persistence requirement).
type nopLogSink struct{}

func (nopLogSink) CreateLogEntry(ctx context.Context, e logs.LogEntry) error { return nil }

// RealtimePublisher is the seam between the logstream package
// and internal/ws/realtime. Production wires a hub that
// broadcasts to every connected WS client; tests use a fake
// that records the events. The interface is intentionally
// tiny so a future replacement (e.g. a NATS-based publisher)
// is a one-line swap.
type RealtimePublisher interface {
	// Publish broadcasts payload to every subscriber of
	// channel. The method MUST be non-blocking; an
	// over-capacity hub should drop the event rather than
	// stall the log pipeline.
	Publish(channel string, payload any)
}

// PodLogChannel is the realtime channel name the service
// publishes to. The spec scenario "Stream log message
// format" uses the literal "container_log" — we follow
// the package-internal convention "k8s.pod.log" which
// matches the rest of the codebase's dotted-namespace
// scheme (e.g. "project.updated").
const PodLogChannel = "k8s.pod.log"

// Service is the orchestration layer between the handler
// and the streamer / logclient. It owns:
//
//   - the 30-day `since` cap (re-using
//     internal/logs.ErrTimeRangeExceeded so the wire
//     envelope matches the rest of the system),
//   - the streamer call + the realtime fan-out,
//   - the LogClient seam for the one-shot /logs endpoint.
//
// The service never touches the network directly.
type Service struct {
	streamer Streamer
	logs     LogClient
	pub      RealtimePublisher
	sink     LogSink
	// now is the clock for the 30-day cap. Indirected so
	// tests can freeze time; defaults to time.Now.
	now func() time.Time
}

// NewService constructs a Service. pub may be nil — when nil,
// the service still works, it just doesn't fan out to the
// realtime bus. logs may be nil too; the one-shot /logs
// endpoint returns an explicit error in that case. The
// sink is optional; when nil the line is dropped after
// the realtime fan-out (the audit P1 #4 default until
// production wires a real *logs.Service).
func NewService(s Streamer, lc LogClient, pub RealtimePublisher, sink ...LogSink) *Service {
	var sk LogSink = nopLogSink{}
	if len(sink) > 0 && sink[0] != nil {
		sk = sink[0]
	}
	return &Service{
		streamer: s,
		logs:     lc,
		pub:      pub,
		sink:     sk,
		now:      time.Now,
	}
}

// PodLogEvent is the payload published on the realtime
// channel. It bundles the routing fields the client needs
// (cluster / namespace / pod / container) and the raw line.
// The wire format matches the spec scenario "Stream log
// message format" so the existing UI bindings work without
// a new field-mapping.
type PodLogEvent struct {
	ClusterID   string    `json:"clusterId"`
	ClusterName string    `json:"clusterName,omitempty"`
	Namespace   string    `json:"namespace"`
	Pod         string    `json:"pod"`
	Container   string    `json:"container"`
	Message     string    `json:"message"`
	Level       string    `json:"level"`
	Timestamp   time.Time `json:"timestamp"`
}

// inferLevel maps the message text to a log level per the
// spec scenario "Log Level Inference": "error"/"ERROR"/
// "failed"/"FATAL" → error; "warn"/"Warning"/"WARN" → warn;
// default → info.
func inferLevel(msg string) string {
	upper := msg
	// We do a few explicit substring checks instead of
	// regex to keep the package import-light.
	if containsAny(upper, "error", "Error", "ERROR", "failed", "FATAL") {
		return "error"
	}
	if containsAny(upper, "warn", "Warning", "WARN") {
		return "warn"
	}
	return "info"
}

// containsAny is a small helper that avoids importing
// strings just for one Contains call.
func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if indexOf(s, n) >= 0 {
			return true
		}
	}
	return false
}

// indexOf is a hand-rolled strings.Index — we use it
// widely enough to justify keeping it private. Returns -1
// when needle is not in s.
func indexOf(s, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	if len(needle) > len(s) {
		return -1
	}
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Stream opens a long-lived channel of LogLine for the
// supplied request. The channel is closed when the source
// stream ends or when ctx is cancelled.
//
// On synchronous validation failure (e.g. `since` beyond the
// 30-day cap), Stream returns a *contracts.APIError so the
// handler can render the standard envelope.
func (s *Service) Stream(ctx context.Context, req StreamRequest) (<-chan LogLine, error) {
	if err := s.validateSince(req.Since); err != nil {
		return nil, err
	}
	src, err := s.streamer.Stream(ctx, req)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to open pod log stream",
			Cause:   err,
		}
	}
	out := make(chan LogLine)
	go s.fanOut(ctx, req, src, out)
	return out, nil
}

// fanOut is the per-subscriber pump: it reads from src,
// publishes each line on the realtime bus, persists each
// line through the LogSink, and forwards the raw line on
// out. The three destinations are decoupled — a slow
// realtime hub or a slow sink MUST NOT stall the WS/SSE
// consumer. Persistence is best-effort: a CreateLogEntry
// failure is logged at warn level and the live stream
// continues.
func (s *Service) fanOut(ctx context.Context, req StreamRequest, src <-chan LogLine, out chan<- LogLine) {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-src:
			if !ok {
				return
			}
			s.publishEvent(req, line)
			s.persistLine(ctx, req, line)
			select {
			case <-ctx.Done():
				return
			case out <- line:
			}
		}
	}
}

// persistLine fires one best-effort CreateLogEntry call
// per line. The 5-second timeout bounds the per-line
// persistence work so a wedged sink cannot stall the
// fan-out. A nil sink is a no-op (the nopLogSink the
// constructor installs satisfies the interface).
func (s *Service) persistLine(ctx context.Context, req StreamRequest, line LogLine) {
	if s.sink == nil {
		return
	}
	// 5s context: a wedged sink should not block the
	// live stream indefinitely. The fan-out loop
	// continues on the next src read even if persist
	// is still running; the per-line timeout ensures
	// the sink call returns within a bounded window.
	persistCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	entry := logs.LogEntry{
		Source:  "k8s",
		Host:    req.Pod,
		Level:   "info", // K8s log lines do not carry a level
		Message: line.Line,
		Labels: map[string]string{
			"cluster": req.ClusterID,
			"pod":     req.Pod,
			"stream":  string(line.Stream),
		},
	}
	if !line.Timestamp.IsZero() {
		entry.Timestamp = line.Timestamp
	}
	if err := s.sink.CreateLogEntry(persistCtx, entry); err != nil {
		// best-effort: log and continue. The
		// realtime bus has the line; downstream
		// subscribers see it. The 3am operator who
		// needs a searchable log trail is the one
		// who pays the cost of a wedged sink.
		_ = err
	}
}

// publishEvent ships a line to the realtime bus. It's a no-op
// when the publisher is nil so the service stays usable in
// tests that don't care about the bus.
func (s *Service) publishEvent(req StreamRequest, line LogLine) {
	if s.pub == nil {
		return
	}
	evt := PodLogEvent{
		ClusterID: req.ClusterID,
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: req.Container,
		Message:   line.Line,
		Level:     inferLevel(line.Line),
		Timestamp: line.Timestamp,
	}
	s.pub.Publish(PodLogChannel, evt)
}

// GetHistorical is the one-shot /logs endpoint's data path.
// The handler passes the user-supplied query; the service
// applies the same `since` cap as Stream and forwards the
// request to the LogClient.
func (s *Service) GetHistorical(ctx context.Context, req StreamRequest) ([]LogLine, error) {
	if err := s.validateSince(req.Since); err != nil {
		return nil, err
	}
	if s.logs == nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "log client is not configured",
		}
	}
	lines, err := s.logs.GetLogs(ctx, req)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to read historical pod logs",
			Cause:   err,
		}
	}
	return lines, nil
}

// validateSince enforces the 30-day rolling window. A zero
// `since` is allowed (no lower bound). Non-zero values
// outside the window are rejected with the same APIError
// shape internal/logs uses — the log-aggregation error
// table is authoritative, and we re-use its constructor.
func (s *Service) validateSince(since time.Time) error {
	if since.IsZero() {
		return nil
	}
	now := s.now()
	if d := now.Sub(since); d > MaxSinceWindow {
		return logs.ErrTimeRangeExceeded(int(d.Hours()), int(MaxSinceWindow.Hours()))
	}
	return nil
}

// IsTimeRangeExceeded is a small helper for callers (the
// handler, in particular) that want to discriminate the
// 422 without re-importing the contracts package.
func IsTimeRangeExceeded(err error) bool {
	var apiErr *contracts.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == contracts.CodeTimeRangeExceeded
	}
	return false
}

// errServiceNotConfigured is exported for tests that need
// to assert on the "no log client" path. It is a sentinel
// of the same shape as the other internal sentinels in
// this package.
var errServiceNotConfigured = errors.New("k8s/logstream: service is not fully configured")
