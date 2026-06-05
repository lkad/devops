package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// serviceFixture builds a Service backed by a fresh
// in-memory repo and a FakeScraper. The Service is the only
// public surface the handler talks to, so a single fixture
// covers the full test surface.
type serviceFixture struct {
	repo    *Repository
	scraper *FakeScraper
	svc     *Service
}

func newServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	repo := NewRepository(openDB(t))
	scraper := NewFakeScraper()
	svc := NewService(repo, scraper)
	return &serviceFixture{repo: repo, scraper: scraper, svc: svc}
}

// TestService_Ingest_HappyPath covers the spec scenario
// "POST /api/v1/metrics — one Metric at a time".
func TestService_Ingest_HappyPath(t *testing.T) {
	f := newServiceFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	out, err := f.svc.Ingest(IngestInput{
		Name:       "cpu",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
		Value:      0.42,
		Timestamp:  ts,
		Labels:     JSONMap{"host": "dev-1"},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if out.ID == "" {
		t.Error("ID was not assigned")
	}
	if out.Name != "cpu" {
		t.Errorf("Name = %q, want cpu", out.Name)
	}
}

// TestService_Ingest_DefaultsTimestamp: a zero Timestamp is
// filled with the current UTC time.
func TestService_Ingest_DefaultsTimestamp(t *testing.T) {
	f := newServiceFixture(t)
	before := time.Now().UTC()
	out, err := f.svc.Ingest(IngestInput{
		Name:       "cpu",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
		Value:      0.5,
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if out.Timestamp.Before(before) {
		t.Errorf("Timestamp = %v, want >= %v", out.Timestamp, before)
	}
}

// TestService_Ingest_RejectsEmptyName is a validation case.
func TestService_Ingest_RejectsEmptyName(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
		Value:      0.5,
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeValidation)
	}
}

// TestService_Ingest_RejectsUnknownTargetType: the service
// is the gatekeeper for the enum.
func TestService_Ingest_RejectsUnknownTargetType(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu",
		TargetType: "vm",
		TargetID:   "dev-1",
		Value:      0.5,
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeValidation)
	}
}

// TestService_Ingest_RejectsEmptyTargetID.
func TestService_Ingest_RejectsEmptyTargetID(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.svc.Ingest(IngestInput{
		Name:       "cpu",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "",
		Value:      0.5,
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeValidation)
	}
}

// TestService_List_HappyPath returns the data.
func TestService_List_HappyPath(t *testing.T) {
	f := newServiceFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	if _, err := f.svc.Ingest(IngestInput{
		Name: "cpu", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.1, Timestamp: ts,
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, err := f.svc.Ingest(IngestInput{
		Name: "mem", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.2, Timestamp: ts.Add(time.Minute),
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	rows, total, err := f.svc.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

// TestService_List_RejectsRangeOver90Days: per the spec, a
// time range longer than 90 days is rejected with
// INVALID_STATE (422).
func TestService_List_RejectsRangeOver90Days(t *testing.T) {
	f := newServiceFixture(t)
	_, _, err := f.svc.List(ListFilter{
		From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeInvalidState {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeInvalidState)
	}
}

// TestService_List_AcceptsRangeAtLimit: a range of exactly
// 90 days is allowed (the cap is inclusive).
func TestService_List_AcceptsRangeAtLimit(t *testing.T) {
	f := newServiceFixture(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_, _, err := f.svc.List(ListFilter{
		From: now.Add(-MaxQueryRange),
		To:   now,
	})
	if err != nil {
		t.Fatalf("expected nil error for range at limit, got %v", err)
	}
}

// TestService_List_RejectsRangeOverLimitByOne: one
// nanosecond over the cap is still invalid.
func TestService_List_RejectsRangeOverLimitByOne(t *testing.T) {
	f := newServiceFixture(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_, _, err := f.svc.List(ListFilter{
		From: now.Add(-MaxQueryRange - time.Nanosecond),
		To:   now,
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeInvalidState {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeInvalidState)
	}
}

// TestService_List_OneSidedRangeNeverTooLong: an open-ended
// range (From or To zero) is always valid — the cap only
// applies when both bounds are set.
func TestService_List_OneSidedRangeNeverTooLong(t *testing.T) {
	f := newServiceFixture(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	// From-only, far in the past.
	if _, _, err := f.svc.List(ListFilter{From: now.Add(-1000 * 24 * time.Hour)}); err != nil {
		t.Errorf("From-only range must not be rejected, got %v", err)
	}
	// To-only, far in the future.
	if _, _, err := f.svc.List(ListFilter{To: now.Add(1000 * 24 * time.Hour)}); err != nil {
		t.Errorf("To-only range must not be rejected, got %v", err)
	}
}

// TestService_GetSeries_HappyPath returns a single
// MetricSeries with data points.
func TestService_GetSeries_HappyPath(t *testing.T) {
	f := newServiceFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		_, err := f.svc.Ingest(IngestInput{
			Name:       "cpu",
			TargetType: string(TargetPhysicalHost),
			TargetID:   "dev-1",
			Value:      float64(i),
			Timestamp:  ts.Add(time.Duration(i) * time.Minute),
			Labels:     JSONMap{"host": "dev-1"},
		})
		if err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	series, err := f.svc.GetSeries(SeriesKey{
		Name:       "cpu",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
	}, ListFilter{})
	if err != nil {
		t.Fatalf("getSeries: %v", err)
	}
	if series.Name != "cpu" {
		t.Errorf("Name = %q, want cpu", series.Name)
	}
	if len(series.Points) != 3 {
		t.Errorf("Points = %d, want 3", len(series.Points))
	}
}

// TestService_GetSeries_RejectsRangeOver90Days: same
// 90-day cap as List.
func TestService_GetSeries_RejectsRangeOver90Days(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.svc.GetSeries(SeriesKey{
		Name:       "cpu",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
	}, ListFilter{
		From: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeInvalidState {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeInvalidState)
	}
}

// TestService_GetSeries_RejectsInvalidKey covers the
// validation of the series key (Name / TargetType / TargetID
// must all be non-empty and TargetType must be known).
func TestService_GetSeries_RejectsInvalidKey(t *testing.T) {
	f := newServiceFixture(t)
	_, err := f.svc.GetSeries(SeriesKey{
		Name:       "",
		TargetType: string(TargetPhysicalHost),
		TargetID:   "dev-1",
	}, ListFilter{})
	var apiErr *contracts.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("Code = %q, want %q", apiErr.Code, contracts.CodeValidation)
	}
}

// TestService_ListSeries_HappyPath returns the unique
// (name, target_type, target_id, labels) tuples.
func TestService_ListSeries_HappyPath(t *testing.T) {
	f := newServiceFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	_, _ = f.svc.Ingest(IngestInput{
		Name: "cpu", TargetType: string(TargetPhysicalHost), TargetID: "dev-1",
		Value: 0.1, Timestamp: ts, Labels: JSONMap{"host": "dev-1"},
	})
	_, _ = f.svc.Ingest(IngestInput{
		Name: "mem", TargetType: string(TargetPhysicalHost), TargetID: "dev-1",
		Value: 0.2, Timestamp: ts, Labels: JSONMap{"host": "dev-1"},
	})
	series, err := f.svc.ListSeries(ListFilter{})
	if err != nil {
		t.Fatalf("listSeries: %v", err)
	}
	if len(series) != 2 {
		t.Errorf("series = %d, want 2", len(series))
	}
}

// TestService_Scrape_IngestRoundTrip: pulling from the
// scraper and ingesting the result must produce the same
// rows as if they were ingested directly. The integration
// is what the monitor loop relies on.
func TestService_Scrape_IngestRoundTrip(t *testing.T) {
	f := newServiceFixture(t)
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	f.scraper.Script("dev-1", []Metric{
		{Name: "cpu", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.1, Timestamp: ts},
		{Name: "mem", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.5, Timestamp: ts},
	}, nil)

	rows, err := f.svc.Scrape(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}
