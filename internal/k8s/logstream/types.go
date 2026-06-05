// Package logstream implements real-time K8s pod log streaming
// for the DevOps Toolkit. The package is layered:
//
//	handler → service → streamer / logclient
//
// The handler translates HTTP / WebSocket / SSE requests into a
// service call; the service validates the request, looks up the
// cluster, and orchestrates the streamer; the streamer is the seam
// that talks to the K8s apiserver (or, in tests, a fake).
//
// All four spec scenarios that the brief enumerates are covered
// by tests in this package: tail with since, fan-out to
// subscribers, real-time over WS, and real-time over SSE.
package logstream

import "time"

// StreamKind identifies the in-cluster source of a log line.
// The wire enum is the same as the spec's "stream" field.
type StreamKind string

const (
	// StreamStdout is the K8s log stream for container stdout.
	StreamStdout StreamKind = "stdout"
	// StreamStderr is the K8s log stream for container stderr.
	StreamStderr StreamKind = "stderr"
)

// MaxSinceWindow is the rolling window the K8s log-aggregation
// pipeline can serve — Loki's hard 30-day cap. The service
// rejects any StreamRequest whose `since` falls outside this
// window with `internal/logs.CodeTimeRangeExceeded` (HTTP 422),
// per the playbook's "AVOID These Conflicts" rule and the
// log-aggregation error table.
const MaxSinceWindow = 30 * 24 * time.Hour

// DefaultMaxBackpressureLines is the channel buffer used when a
// streamer hands lines to a slow consumer. When the buffer
// fills, the oldest line is dropped and a `dropped` counter is
// incremented on the last event sent to the consumer.
const DefaultMaxBackpressureLines = 1000

// DefaultMaxLines is the cap applied to TailLines when a caller
// asks for "up to N" but doesn't set N (or sets N <= 0). The
// one-shot /logs endpoint uses this default.
const DefaultMaxLines = 500

// StreamRequest is the service-layer input shape. The handler
// builds it from URL params + query string; the service then
// fills defaults, validates, and passes it to the streamer.
//
// `Container` is optional — an empty value means "all
// containers in the pod", matching the spec scenario
// "Stream all containers".
type StreamRequest struct {
	ClusterID string
	Namespace string
	Pod       string
	Container string

	// Since is the RFC3339 / time.Time of the oldest log line
	// the caller wants. Zero means "no lower bound"; non-zero
	// values older than MaxSinceWindow are rejected.
	Since time.Time

	// TailLines caps the historical tail. Zero / negative means
	// "use DefaultMaxLines". Honoured by the one-shot endpoint
	// and the initial backlog of the streaming endpoints.
	TailLines int

	// Follow enables the long-lived streaming mode. When false
	// the streamer returns the historical tail and closes the
	// channel.
	Follow bool
}

// LogLine is a single log line. Stream is always "stdout" or
// "stderr"; Timestamp is the K8s-reported wall-clock time
// (zero when the source is a syslog / raw file without one).
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	Stream    StreamKind `json:"stream"`
}

// LogFrame is what the handler sends over WS / SSE. It bundles
// a LogLine with the routing fields the client needs to
// de-multiplex, and a `dropped` counter that the backpressure
// policy increments on the LAST frame before a drop so the
// client can show a "X lines were dropped" warning.
//
// `Dropped` is a running total — it never resets between frames
// in a single stream session.
type LogFrame struct {
	ClusterID string     `json:"clusterId"`
	Namespace string     `json:"namespace"`
	Pod       string     `json:"pod"`
	Container string     `json:"container,omitempty"`
	Line      LogLine    `json:"line"`
	Dropped   int64      `json:"dropped"`
}
