// Package observability provides Prometheus export for the
// DevOps Toolkit backend. It exports five signals:
//
//   - http_requests_total  (counter, by route + status)
//   - http_request_duration_seconds (histogram, by route)
//   - monitor_loop_iterations_total (counter)
//   - monitor_loop_errors_total (counter, by host_id)
//   - monitor_loop_last_tick_timestamp_seconds (gauge)
//   - go runtime metrics   (default collectors, goroutines, memstats, etc.)
//
// The package is intentionally minimal: callers wire the
// middleware into the Gin router and mount the /metrics
// handler on a separate mux (the metrics route is not
// itself counted by the middleware — a common Prometheus
// anti-pattern is to scrape a server that counts its own
// scrapes).
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is the registry + instruments. The zero value is
// not usable; construct with New.
type Metrics struct {
	reg             *prometheus.Registry
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	// monitorLoopIterations: total number of monitor-loop
	// ticks (one per periodic pass over the host set).
	monitorLoopIterations prometheus.Counter
	// monitorLoopErrors: per-host check failures, labelled by
	// host_id so an operator can see WHICH host is flapping.
	// host_id has bounded cardinality (one row per physical
	// host) so this is safe to keep as a label.
	monitorLoopErrors *prometheus.CounterVec
	// monitorLoopLastTick: unix-seconds of the start of the
	// most recent tick. A stale value (older than 2*Tick)
	// means the loop is wedged.
	monitorLoopLastTick prometheus.Gauge
}

// New builds a Metrics with a fresh registry (default Go
// collectors included). The returned value is safe for
// concurrent use.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	m := &Metrics{
		reg: reg,
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devops_toolkit",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total HTTP requests served, labelled by route and status code.",
			},
			[]string{"route", "method", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "devops_toolkit",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds, labelled by route and method.",
				// Buckets tuned for an internal API: 5ms .. 5s.
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"route", "method"},
		),
		monitorLoopIterations: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "devops_toolkit",
				Subsystem: "monitor_loop",
				Name:      "iterations_total",
				Help:      "Total number of physical-host monitor-loop ticks.",
			},
		),
		monitorLoopErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devops_toolkit",
				Subsystem: "monitor_loop",
				Name:      "errors_total",
				Help:      "Total physical-host monitor-loop check failures, labelled by host_id.",
			},
			[]string{"host_id"},
		),
		monitorLoopLastTick: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "devops_toolkit",
				Subsystem: "monitor_loop",
				Name:      "last_tick_timestamp_seconds",
				Help:      "Unix-seconds of the start of the most recent physical-host monitor-loop tick.",
			},
		),
	}
	reg.MustRegister(m.requestsTotal, m.requestDuration, m.monitorLoopIterations, m.monitorLoopErrors, m.monitorLoopLastTick)
	return m
}

// Registry returns the underlying Prometheus registry. The
// /metrics handler is built from this.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.reg
}

// Handler returns an http.Handler that serves the metrics in
// Prometheus text format. Mount this on a separate mux (NOT
// under the same Gin router that the middleware is on — see
// package comment).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Middleware returns a Gin middleware that records the
// request count and duration. The route label is the Gin
// matched route template (e.g. "/api/v1/physical-hosts/:id"),
// not the raw URL — high-cardinality labels would blow up
// the metrics storage. Requests that don't match a known
// route (404) are labelled with the literal path to avoid
// an explosion of distinct unknown-route labels.
//
// Recording is deferred to c.Next()'s completion so we
// capture the response status and total handler time.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		m.requestsTotal.WithLabelValues(route, c.Request.Method, status).Inc()
		m.requestDuration.WithLabelValues(route, c.Request.Method).Observe(time.Since(start).Seconds())
	}
}

// IncMonitorLoopIteration bumps the per-tick counter. Called
// at the START of every monitor-loop pass so the counter
// reflects "ticks attempted" not "ticks completed".
func (m *Metrics) IncMonitorLoopIteration() {
	m.monitorLoopIterations.Inc()
}

// SetMonitorLoopLastTick records the wall-clock of the most
// recent tick. A stale value (older than 2*Tick) is the
// signal that the loop is wedged.
func (m *Metrics) SetMonitorLoopLastTick(t time.Time) {
	m.monitorLoopLastTick.Set(float64(t.Unix()))
}

// IncMonitorLoopError bumps the per-host error counter. The
// host_id label keeps cardinality bounded (one row per
// physical host in physical_hosts).
func (m *Metrics) IncMonitorLoopError(hostID string) {
	m.monitorLoopErrors.WithLabelValues(hostID).Inc()
}
