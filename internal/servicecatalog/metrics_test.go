package servicecatalog

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestHealthStatusValue pins the enum-to-number mapping. The
// dashboard depends on these exact values (0=unknown,
// 1=healthy, 2=degraded) — flipping them silently would
// invert every color band in Grafana.
func TestHealthStatusValue(t *testing.T) {
	cases := []struct {
		in   HealthStatus
		want float64
	}{
		{HealthUnknown, 0},
		{HealthHealthy, 1},
		{HealthDegraded, 2},
		{HealthStatus("bogus-future-value"), 0},
	}
	for _, c := range cases {
		if got := healthStatusValue(c.in); got != c.want {
			t.Errorf("healthStatusValue(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestNewMetrics_NilRegistererIsNoop pins the test-friendly
// branch: handlers built without a registry (the unit-test
// path) must not panic on Record/Reset.
func TestNewMetrics_NilRegistererIsNoop(t *testing.T) {
	m := NewMetrics(nil)
	if m == nil {
		t.Fatal("NewMetrics(nil) returned nil")
	}
	// Recording on a nil-registered Metrics still mutates
	// the internal collectors (they exist; they're just
	// not registered). Should not panic.
	m.Record(&Service{ID: "s1", Name: "alpha", Tier: TierStandard}, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})
	m.Reset("s1")
}

// TestMetrics_NilReceiverIsNoop pins the other no-op branch:
// a *Metrics that was never built (callers that did not run
// SetMetrics) must not panic on Record/Reset.
func TestMetrics_NilReceiverIsNoop(t *testing.T) {
	var m *Metrics // nil
	m.Record(&Service{ID: "s1"}, &HealthResult{Status: HealthHealthy})
	m.Reset("s1")
}

// TestMetrics_RecordSetsGauge pins the happy path: Record
// updates the gauge with the numeric status keyed by all
// four labels, and bumps the counter.
func TestMetrics_RecordSetsGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	svc := &Service{ID: "s1", Name: "alpha", Tier: TierCritical}
	res := &HealthResult{Status: HealthDegraded, DerivedFrom: "k8s_pod_health"}
	m.Record(svc, res)

	got := readGauge(t, reg, "devops_toolkit_service_health_status",
		map[string]string{"service_id": "s1", "service_name": "alpha", "tier": "critical", "derived_from": "k8s_pod_health"})
	if got != 2 {
		t.Errorf("gauge = %v, want 2 (degraded)", got)
	}

	cnt := readCounter(t, reg, "devops_toolkit_service_health_rollup_total",
		map[string]string{"status": "degraded", "derived_from": "k8s_pod_health"})
	if cnt != 1 {
		t.Errorf("counter = %v, want 1", cnt)
	}
}

// TestMetrics_RecordOverwritesGauge pins the gauge's
// last-write-wins semantics: a degraded result followed by a
// healthy result for the same service must leave the gauge
// at 1 (healthy), not 2 (the old degraded value).
func TestMetrics_RecordOverwritesGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	svc := &Service{ID: "s1", Name: "alpha", Tier: TierStandard}

	m.Record(svc, &HealthResult{Status: HealthDegraded, DerivedFrom: "last_pipeline_run"})
	m.Record(svc, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})

	got := readGauge(t, reg, "devops_toolkit_service_health_status",
		map[string]string{"service_id": "s1", "service_name": "alpha", "tier": "standard", "derived_from": "last_pipeline_run"})
	if got != 1 {
		t.Errorf("gauge = %v, want 1 (latest = healthy)", got)
	}
}

// TestMetrics_CounterAccumulates pins counter semantics: two
// rollups with the same labels increment to 2, NOT overwrite.
// Operators read this as "we computed N rollups producing
// status X" over the dashboard window.
func TestMetrics_CounterAccumulates(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	svc := &Service{ID: "s1", Name: "alpha", Tier: TierStandard}
	res := &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"}

	m.Record(svc, res)
	m.Record(svc, res)
	m.Record(svc, res)

	cnt := readCounter(t, reg, "devops_toolkit_service_health_rollup_total",
		map[string]string{"status": "healthy", "derived_from": "last_pipeline_run"})
	if cnt != 3 {
		t.Errorf("counter = %v, want 3", cnt)
	}
}

// TestMetrics_RecordNilArgsAreNoop pins the defensive branch:
// passing a nil Service or nil HealthResult must not panic
// and must not emit a series (we'd otherwise get a series
// with empty labels in production).
func TestMetrics_RecordNilArgsAreNoop(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.Record(nil, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})
	m.Record(&Service{ID: "s1"}, nil)
	m.Record(nil, nil)

	// No series should have been emitted. Gather and
	// confirm both metric families have no samples.
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if len(mf.GetMetric()) > 0 {
			t.Errorf("expected no samples, found %d in %s", len(mf.GetMetric()), mf.GetName())
		}
	}
}

// TestMetrics_ResetDropsSeries pins the deletion path: after
// Reset, the gauge series for that service_id must be gone
// (so a deleted service does not linger in Prometheus). The
// counter, by design, stays — it represents observed history.
func TestMetrics_ResetDropsSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	svc := &Service{ID: "s1", Name: "alpha", Tier: TierStandard}

	m.Record(svc, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})
	m.Reset("s1")

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "devops_toolkit_service_health_status" {
			if n := len(mf.GetMetric()); n != 0 {
				t.Errorf("gauge series after Reset = %d, want 0", n)
			}
		}
		if mf.GetName() == "devops_toolkit_service_health_rollup_total" {
			// Counter must survive Reset.
			if n := len(mf.GetMetric()); n != 1 {
				t.Errorf("counter series after Reset = %d, want 1 (counter must persist)", n)
			}
		}
	}
}

// TestMetrics_ResetEmptyIDIsNoop pins the guard against an
// accidental wildcard delete: Reset("") must NOT drop every
// series in the gauge (which DeletePartialMatch with an empty
// match would otherwise do).
func TestMetrics_ResetEmptyIDIsNoop(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	m.Record(&Service{ID: "s1", Name: "alpha", Tier: TierStandard}, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})
	m.Record(&Service{ID: "s2", Name: "beta", Tier: TierCritical}, &HealthResult{Status: HealthDegraded, DerivedFrom: "k8s_pod_health"})

	m.Reset("")

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "devops_toolkit_service_health_status" {
			if n := len(mf.GetMetric()); n != 2 {
				t.Errorf("gauge series after Reset(\"\") = %d, want 2 (no wildcard delete)", n)
			}
		}
	}
}

// TestMetrics_RegisteredMetricsExposedInGather verifies the
// Prometheus gather payload contains both expected metric
// families with the right help strings. This catches
// rename / namespace-mismatch bugs that would otherwise only
// surface when the dashboard query returns "No data".
//
// Registry.Gather() only includes families that have at least
// one observed sample, so we Record one observation before
// asking — otherwise the bare-registered families would not
// show up and the assertion would be vacuous.
func TestMetrics_RegisteredMetricsExposedInGather(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	m.Record(&Service{ID: "s1", Name: "alpha", Tier: TierStandard}, &HealthResult{Status: HealthHealthy, DerivedFrom: "last_pipeline_run"})

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	wantNames := map[string]bool{
		"devops_toolkit_service_health_status":       true,
		"devops_toolkit_service_health_rollup_total": true,
	}
	for _, mf := range mfs {
		delete(wantNames, mf.GetName())
		// Help string must mention the metric purpose (a
		// missing Help would mean the dashboard's hover
		// tooltip is blank).
		if !strings.Contains(mf.GetHelp(), "health") && !strings.Contains(mf.GetHelp(), "Health") {
			t.Errorf("metric %s help string = %q (missing 'health')", mf.GetName(), mf.GetHelp())
		}
	}
	if len(wantNames) > 0 {
		t.Errorf("missing metric families: %v", wantNames)
	}
}

// readGauge pulls the gauge value with the given labels from
// a Prometheus registry. Returns 0 and t.Errorf if the series
// isn't present.
func readGauge(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetGauge().GetValue()
			}
		}
	}
	t.Errorf("gauge %s with labels %v not found", name, labels)
	return 0
}

// readCounter pulls the counter value with the given labels.
func readCounter(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	t.Errorf("counter %s with labels %v not found", name, labels)
	return 0
}

func matchLabels(got []*dto.LabelPair, want map[string]string) bool {
	have := make(map[string]string, len(got))
	for _, p := range got {
		have[p.GetName()] = p.GetValue()
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
