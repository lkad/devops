// Package servicecatalog Prometheus metrics for the service
// health rollup. The metrics live alongside the catalog
// because they describe domain state (per-service health), not
// transport-level state (HTTP traffic, latency) — those live
// in internal/observability.
package servicecatalog

import (
	"github.com/prometheus/client_golang/prometheus"
)

// healthStatusValue maps the package's HealthStatus enum into
// the numeric value the gauge reports. The mapping is
// intentionally NOT a bare iota cast on HealthStatus — the
// string-typed enum has no fixed ordering and an iota cast
// would silently shuffle if someone reorders the consts. An
// explicit table is the only safe form.
//
// Reserved values for future expansion:
//
//	0 = unknown   (no signal at all)
//	1 = healthy   (last run succeeded / pods ready)
//	2 = degraded  (last run failed / pods below replicas)
//	3 = down      (reserved — Health does not emit this today;
//	               a future "circuit-breaker / paged" state
//	               would land here)
func healthStatusValue(s HealthStatus) float64 {
	switch s {
	case HealthHealthy:
		return 1
	case HealthDegraded:
		return 2
	case HealthUnknown:
		return 0
	default:
		return 0
	}
}

// Metrics is the Prometheus instrument set for the service
// catalog. Build with NewMetrics; the registerer is usually
// the observability package's shared registry so all
// devops_toolkit_* metrics share one /metrics endpoint.
//
// Cardinality budget:
//
//	~30 services × 3 tiers × 2 derived_from = ~180 series.
//	Renaming a service breaks the time-series (Prometheus
//	treats new labels as new series); for an internal
//	platform with stable service names that is acceptable.
//	If services ever number 1000+, drop service_name from
//	the gauge and require operators to join with the
//	services table in Grafana.
type Metrics struct {
	healthStatus *prometheus.GaugeVec
	rollupTotal  *prometheus.CounterVec
}

// NewMetrics builds and registers the instrument set against
// the supplied registerer. Passing a nil registerer makes
// NewMetrics a no-op — every method on the returned *Metrics
// is safe to call but does not record anything. Tests that
// don't care about the metric payload can pass nil; production
// passes observability.Metrics.Registry().
//
// Registering twice on the same registerer panics (the
// prometheus library's contract); guard against that at the
// call site, not here.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		healthStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "devops_toolkit",
				Subsystem: "service",
				Name:      "health_status",
				Help:      "Latest derived health for each service. 0=unknown, 1=healthy, 2=degraded. Updated on every GET /services/:id/health.",
			},
			[]string{"service_id", "service_name", "tier", "derived_from"},
		),
		rollupTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devops_toolkit",
				Subsystem: "service",
				Name:      "health_rollup_total",
				Help:      "Count of health-rollup evaluations, labelled by resulting status and the signal that produced it.",
			},
			[]string{"status", "derived_from"},
		),
	}
	if reg != nil {
		reg.MustRegister(m.healthStatus, m.rollupTotal)
	}
	return m
}

// Record updates the gauge with the latest rollup status for
// the given service and bumps the rollup counter. Safe to
// call from any goroutine; the underlying prometheus
// instruments are concurrency-safe.
//
// A nil receiver is a no-op so callers that wired the handler
// without a Metrics instance (unit tests) do not need a
// branch at every call site.
func (m *Metrics) Record(svc *Service, result *HealthResult) {
	if m == nil || svc == nil || result == nil {
		return
	}
	m.healthStatus.WithLabelValues(
		svc.ID,
		svc.Name,
		string(svc.Tier),
		result.DerivedFrom,
	).Set(healthStatusValue(result.Status))
	m.rollupTotal.WithLabelValues(
		string(result.Status),
		result.DerivedFrom,
	).Inc()
}

// Reset clears the gauge for a specific service. Use after a
// service is deleted so its stale "last known state" series
// does not linger in Prometheus until the next scrape eviction.
// Counter values are not reset (they represent observed history,
// not state). A nil receiver is a no-op.
func (m *Metrics) Reset(serviceID string) {
	if m == nil || serviceID == "" {
		return
	}
	m.healthStatus.DeletePartialMatch(prometheus.Labels{"service_id": serviceID})
}
