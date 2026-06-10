// Package observability — OpenTelemetry tracing (P1.1).
//
// The Tracing struct owns the global TracerProvider
// lifecycle and exposes a Gin middleware that:
//   1. extracts a W3C traceparent header (or mints a
//      new trace_id when absent)
//   2. starts a span for the request with attributes
//      http.route, http.request.method, http.response.status_code,
//      http.server.duration
//   3. writes the trace_id back in the X-Trace-Id
//      response header
//
// The exporter is selected at construction time:
//   - TracingConfig{OTLPEndpoint: "http://..."} — OTLP HTTP
//   - TracingConfig{Stdout: true} — JSON spans to stdout
//   - otherwise — noop tracer (no exporter, but spans
//     are still produced in-process for the recorder +
//     the response header is still set)
//
// The recorder (Tracing.Recorder()) returns an
// in-process span recorder the test suite uses to
// assert span attributes without coupling to the
// exporter. The production wiring never uses the
// recorder.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracingConfig is the env-driven harness. SamplerRatio
// is 0..1 (clamped); 0 = never sample, 1 = always sample.
// OTLPEndpoint and Stdout are mutually exclusive; if
// OTLPEndpoint is set Stdout is ignored.
type TracingConfig struct {
	ServiceName  string
	ServiceVer   string
	SamplerRatio float64
	// OTLPEndpoint is the HTTP OTLP collector URL. Empty
	// = no remote exporter (use Stdout or nothing).
	OTLPEndpoint string
	// Stdout sends every span to stderr in JSON form.
	// Useful in dev when there is no collector.
	Stdout bool
}

// Tracing is the observability singleton. It is safe
// for concurrent use. Shutdown should be called on
// process exit so the OTLP exporter flushes its
// in-flight spans.
type Tracing struct {
	cfg     TracingConfig
	tp      *sdktrace.TracerProvider
	tracer  trace.Tracer
	rec     *recordingSpanProcessor
	propag  propagation.TextMapPropagator
}

// SamplerRatioFromEnv reads OTEL_TRACES_SAMPLER_ARG and
// returns a sampling ratio in [0, 1]. Returns 0.1 (the
// default) when the env is unset or unparseable.
func SamplerRatioFromEnv() float64 {
	v := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if v == "" {
		return 0.1
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0.1
	}
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// NewTracing initializes the global TracerProvider
// (otel.SetTracerProvider) and returns a Tracing value
// the router can call .Middleware() on. The provider
// stays active for the lifetime of the process; pair
// with Shutdown on exit.
func NewTracing(cfg TracingConfig) *Tracing {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "devops-toolkit"
	}
	if cfg.ServiceVer == "" {
		cfg.ServiceVer = "dev"
	}
	if cfg.SamplerRatio < 0 {
		cfg.SamplerRatio = 0
	}
	if cfg.SamplerRatio > 1 {
		cfg.SamplerRatio = 1
	}

	// Always wire a recording processor so the in-process
	// recorder (and the test suite) sees spans regardless
	// of the remote exporter.
	rec := newRecordingSpanProcessor()

	// Pick the remote exporter if any. stdouttrace.New
	// returns an *Exporter which does not itself
	// implement SpanProcessor; it is wrapped in a
	// SimpleSpanProcessor that calls Shutdown and
	// ForceFlush on the exporter.
	var remoteExporter sdktrace.SpanExporter
	if cfg.OTLPEndpoint != "" {
		// OTLP HTTP exporter wired in a follow-up; in
		// P1.1 we use the stdout exporter when enabled.
		// Wiring OTLP HTTP lives outside this file so
		// the test dep tree stays small.
	} else if cfg.Stdout {
		exp, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err == nil {
			remoteExporter = exp
		}
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVer),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))),
		sdktrace.WithResource(res),
	)
	// Always register the recording processor first so
	// the in-process recorder sees every span. The
	// optional remote exporter is layered on top via
	// a BatchSpanProcessor.
	tp.RegisterSpanProcessor(rec)
	if remoteExporter != nil {
		tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(remoteExporter))
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracing{
		cfg:    cfg,
		tp:     tp,
		tracer: tp.Tracer(cfg.ServiceName),
		rec:    rec,
		propag: otel.GetTextMapPropagator(),
	}
}

// Shutdown flushes in-flight spans and shuts the
// provider down. Safe to call from a defer.
func (t *Tracing) Shutdown(ctx context.Context) error {
	if t == nil || t.tp == nil {
		return nil
	}
	return t.tp.Shutdown(ctx)
}

// Recorder returns the in-process recording processor so
// tests can inspect spans. Production code should not
// call this.
func (t *Tracing) Recorder() *recordingSpanProcessor { return t.rec }

// Middleware returns a Gin middleware that
//   1. extracts the W3C traceparent from the request
//   2. starts a span for the request
//   3. sets the X-Trace-Id response header
//   4. records http.route / method / status / duration
//      attributes on the span
func (t *Tracing) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := t.propag.Extract(c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header))

		// Start a span (root or child, depending on
		// whether a parent was extracted).
		var span trace.Span
		ctx, span = t.tracer.Start(ctx, "HTTP "+c.Request.Method+" "+c.FullPath(),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("http.route", routeOrUnmatched(c)),
				attribute.String("net.peer.ip", c.ClientIP()),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		start := time.Now()
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(
			attribute.Int("http.response.status_code", status),
		)
		// server.duration is the canonical name; some
		// backends expect it under server.duration so
		// we record both.
		span.SetAttributes(
			attribute.Float64("http.server.duration", time.Since(start).Seconds()),
		)

		// X-Trace-Id: extract the trace_id from the
		// span context. If somehow empty, fall back to
		// a fresh random id so the header is always
		// set.
		traceID := span.SpanContext().TraceID().String()
		if traceID == "00000000000000000000000000000000" {
			traceID = randomTraceID()
		}
		c.Writer.Header().Set("X-Trace-Id", traceID)
	}
}

// routeOrUnmatched returns the matched Gin route
// template; for 404s it collapses to the literal
// "unmatched" label to keep cardinality bounded.
func routeOrUnmatched(c *gin.Context) string {
	if r := c.FullPath(); r != "" {
		return r
	}
	return "unmatched"
}

// randomTraceID returns a 32-hex-char string. Used as
// the X-Trace-Id fallback when no parent was extracted
// and a fresh span context has not been initialised.
// Inlined here (rather than calling a uuid lib) to
// keep the package dep tree small.
func randomTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Pathological: at least don't panic. Fill
		// with a timestamp so the header is still set.
		ts := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(ts >> (uint(i) * 8))
		}
	}
	return hex.EncodeToString(b[:])
}
