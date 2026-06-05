package metrics

import (
	"errors"
	"testing"
	"time"
)

// repoFixture builds a Repository backed by a fresh in-memory
// DB. Mirrors the physicalhost package's repoFixture pattern.
func repoFixture(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(openDB(t))
}

// sampleMetric builds a metric with a fixed (i) suffix on the
// name + target so list-filter tests can tell rows apart.
func sampleMetric(name, targetType, targetID string, ts time.Time, v float64) *Metric {
	return &Metric{
		Name:       name,
		TargetType: targetType,
		TargetID:   targetID,
		Value:      v,
		Timestamp:  ts,
		Labels:     JSONMap{"host": targetID},
	}
}

// TestRepository_CreateAndGet is the minimum persistence contract.
func TestRepository_CreateAndGet(t *testing.T) {
	repo := repoFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	m := sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", ts, 0.42)
	if err := repo.Create(m); err != nil {
		t.Fatalf("create: %v", err)
	}
	if m.ID == "" {
		t.Fatal("ID was not assigned")
	}

	got, err := repo.Get(m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "cpu" {
		t.Errorf("Name = %q, want cpu", got.Name)
	}
	if got.Value != 0.42 {
		t.Errorf("Value = %v, want 0.42", got.Value)
	}
	if !got.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, ts)
	}
	if got.Labels["host"] != "dev-1" {
		t.Errorf("Labels[host] = %v, want dev-1", got.Labels["host"])
	}
}

// TestRepository_Get_NotFound must return the typed sentinel.
func TestRepository_Get_NotFound(t *testing.T) {
	repo := repoFixture(t)
	_, err := repo.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing metric")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestRepository_List_NoFilter returns every row.
func TestRepository_List_NoFilter(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(time.Duration(i)*time.Minute), float64(i)))
	}
	rows, total, err := repo.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestRepository_List_FilterByName narrows by metric name.
func TestRepository_List_FilterByName(t *testing.T) {
	repo := repoFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", ts, 1))
	_ = repo.Create(sampleMetric("mem", string(TargetPhysicalHost), "dev-1", ts, 2))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-2", ts, 3))

	rows, total, err := repo.List(ListFilter{Name: "cpu"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	for _, r := range rows {
		if r.Name != "cpu" {
			t.Errorf("got name %q, want cpu", r.Name)
		}
	}
}

// TestRepository_List_FilterByTarget narrows by (target_type,
// target_id). Combined-filter semantics are part of the
// public API.
func TestRepository_List_FilterByTarget(t *testing.T) {
	repo := repoFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", ts, 1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-2", ts, 2))
	_ = repo.Create(sampleMetric("cpu", string(TargetK8sPod), "pod-1", ts, 3))

	rows, total, err := repo.List(ListFilter{
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%d, want 1/1", total, len(rows))
	}
	if rows[0].TargetID != "dev-1" {
		t.Errorf("TargetID = %q, want dev-1", rows[0].TargetID)
	}
}

// TestRepository_List_FilterByTimeRange applies the
// (From, To) bounds. The repository must treat From as
// inclusive and To as exclusive.
func TestRepository_List_FilterByTimeRange(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(-2*time.Hour), 1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(-1*time.Hour), 2))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base, 3))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(1*time.Hour), 4))

	rows, total, err := repo.List(ListFilter{
		From: base.Add(-1 * time.Hour),
		To:   base, // exclusive
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// The window is [-1h, 0). Only the -2h row is excluded; the
	// 0h row is excluded because To is exclusive; -1h and
	// 1h are outside the window. Wait — -1h is >= From so
	// it's in, and 1h is > To so it's out. The 0h row is at
	// exactly the To boundary (exclusive), so it's out.
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%d, want 1/1", total, len(rows))
	}
	if rows[0].Value != 2 {
		t.Errorf("Value = %v, want 2", rows[0].Value)
	}
}

// TestRepository_List_TimeRange_OnlyFrom / To alone are also
// valid. From-only means "from this point on", To-only means
// "up to this point".
func TestRepository_List_TimeRange_OnlyFrom(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(-1*time.Hour), 1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base, 2))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(1*time.Hour), 3))

	rows, _, err := repo.List(ListFilter{From: base})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2 (From is inclusive)", len(rows))
	}
}

func TestRepository_List_TimeRange_OnlyTo(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(-1*time.Hour), 1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base, 2))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(1*time.Hour), 3))

	rows, _, err := repo.List(ListFilter{To: base})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows = %d, want 1 (To is exclusive)", len(rows))
	}
}

// TestRepository_List_Pagination verifies limit/offset yield a
// stable page.
func TestRepository_List_Pagination(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev", base.Add(time.Duration(i)*time.Minute), float64(i)))
	}
	rows, total, err := repo.List(ListFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 7 {
		t.Errorf("total = %d, want 7", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestRepository_ListSeries_Flat covers the "list unique
// series" case. Three rows for (cpu, dev-1) and one row for
// (mem, dev-1) yield two distinct series.
func TestRepository_ListSeries_Flat(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base, 0.1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base.Add(time.Minute), 0.2))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base.Add(2*time.Minute), 0.3))
	_ = repo.Create(sampleMetric("mem", string(TargetPhysicalHost), "dev-1", base, 0.5))

	series, err := repo.ListSeries(ListFilter{})
	if err != nil {
		t.Fatalf("listSeries: %v", err)
	}
	if len(series) != 2 {
		t.Errorf("series count = %d, want 2", len(series))
	}
}

// TestRepository_GetSeriesByName returns the data points for
// one (name, target_type, target_id) tuple ordered by
// timestamp ascending.
func TestRepository_GetSeriesByName(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	// Insert in non-monotonic order; the repository must
	// sort by timestamp.
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base.Add(2*time.Minute), 0.3))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base, 0.1))
	_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base.Add(time.Minute), 0.2))

	points, err := repo.GetSeriesByName("cpu", string(TargetPhysicalHost), "dev-1", ListFilter{})
	if err != nil {
		t.Fatalf("getSeries: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("points = %d, want 3", len(points))
	}
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp.Before(points[i-1].Timestamp) {
			t.Errorf("points[%d] (%v) before points[%d] (%v)", i, points[i].Timestamp, i-1, points[i-1].Timestamp)
		}
	}
	if points[0].Value != 0.1 {
		t.Errorf("points[0].Value = %v, want 0.1", points[0].Value)
	}
}

// TestRepository_GetSeriesByName_Empty is a missing series —
// not an error, just an empty slice.
func TestRepository_GetSeriesByName_Empty(t *testing.T) {
	repo := repoFixture(t)
	points, err := repo.GetSeriesByName("nope", string(TargetPhysicalHost), "missing", ListFilter{})
	if err != nil {
		t.Fatalf("getSeries: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("points = %d, want 0", len(points))
	}
}

// TestRepository_GetSeriesByName_RespectsTimeRange narrows
// the data points to a sub-window.
func TestRepository_GetSeriesByName_RespectsTimeRange(t *testing.T) {
	repo := repoFixture(t)
	base := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		_ = repo.Create(sampleMetric("cpu", string(TargetPhysicalHost), "dev-1", base.Add(time.Duration(i)*time.Minute), float64(i)))
	}
	points, err := repo.GetSeriesByName("cpu", string(TargetPhysicalHost), "dev-1", ListFilter{
		From: base.Add(time.Minute),
		To:   base.Add(3 * time.Minute), // exclusive
	})
	if err != nil {
		t.Fatalf("getSeries: %v", err)
	}
	if len(points) != 2 {
		t.Errorf("points = %d, want 2", len(points))
	}
}
