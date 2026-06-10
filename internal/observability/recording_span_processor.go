package observability

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/sdk/trace"
	ottrace "go.opentelemetry.io/otel/trace"
)

// recordingSpanProcessor is an in-process SpanProcessor
// used by tests to inspect spans without coupling to the
// exporter. Production code never calls Collect; the
// recorder is exposed for the test suite.
type recordingSpanProcessor struct {
	mu    sync.Mutex
	spans []recordedSpan
}

// recordedSpan is the minimal projection tests need:
// just the attributes. The full ReadOnlySpan interface
// has dozens of methods we do not need.
type recordedSpan struct {
	Name       string
	Attributes []attributeKV
	TraceID    string
	SpanID     string
}

// attributeKV is a flat (key, string-value) projection
// of the OTel attribute set, easier to assert against
// than the SDK's typed attribute.KeyValue type.
type attributeKV struct {
	Key   string
	Value string
}

// newRecordingSpanProcessor is the constructor.
func newRecordingSpanProcessor() *recordingSpanProcessor {
	return &recordingSpanProcessor{}
}

// OnStart satisfies trace.SpanProcessor.
func (r *recordingSpanProcessor) OnStart(_ context.Context, _ trace.ReadWriteSpan) {}

// OnEnd satisfies trace.SpanProcessor. We pull the
// attributes the tests actually assert on, and discard
// the rest of the ReadOnlySpan surface.
func (r *recordingSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
	rs := recordedSpan{
		Name:    s.Name(),
		TraceID: s.SpanContext().TraceID().String(),
		SpanID:  s.SpanContext().SpanID().String(),
	}
	for _, kv := range s.Attributes() {
		rs.Attributes = append(rs.Attributes, attributeKV{
			Key:   string(kv.Key),
			Value: kv.Value.Emit(),
		})
	}
	r.mu.Lock()
	r.spans = append(r.spans, rs)
	r.mu.Unlock()
}

// Shutdown satisfies trace.SpanProcessor.
func (r *recordingSpanProcessor) Shutdown(_ context.Context) error { return nil }

// ForceFlush satisfies trace.SpanProcessor.
func (r *recordingSpanProcessor) ForceFlush(_ context.Context) error { return nil }

// Collect returns a snapshot of the spans recorded so
// far. Tests use this to assert on attributes; the
// production wiring does not call it.
func (r *recordingSpanProcessor) Collect() []recordedSpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedSpan, len(r.spans))
	copy(out, r.spans)
	return out
}

// AsStringMap flattens the attributes of one recorded
// span into a map[string]string so tests can assert on
// keys without walking the attributeKV slice. The
// returned map is a copy; callers may mutate it.
func (s recordedSpan) AsStringMap() map[string]string {
	out := make(map[string]string, len(s.Attributes))
	for _, kv := range s.Attributes {
		out[kv.Key] = kv.Value
	}
	return out
}

// ensure the imports compile: ottrace is imported in case
// future code wants to assert on a trace.SpanContext.
var _ = ottrace.SpanContextFromContext
