package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Service is the business-logic layer for the metrics
// subsystem. It owns validation (target_type enum, time-
// range cap), the ingest / list / get-series orchestration,
// and the scraper dispatch. The repository is a pure data
// layer; the handler is a pure transport layer.
type Service struct {
	repo    *Repository
	scraper Scraper
	// now is the clock dependency. Tests can inject a
	// fixed clock by passing NewServiceWithClock; production
	// uses the wall clock via NewService.
	now func() time.Time
}

// NewService builds a Service with the wall clock and the
// supplied scraper. The scraper is used by Service.Scrape
// to pull a fresh batch of observations from a target.
func NewService(repo *Repository, scraper Scraper) *Service {
	return NewServiceWithClock(repo, scraper, time.Now)
}

// NewServiceWithClock is the clock-injecting constructor.
// Tests use it to make Ingest's default-timestamp path
// deterministic.
func NewServiceWithClock(repo *Repository, scraper Scraper, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, scraper: scraper, now: now}
}

// SeriesKey is the (name, target_type, target_id) tuple
// that identifies a single time-series. The handler
// decodes the URL into this struct.
type SeriesKey struct {
	Name       string
	TargetType string
	TargetID   string
}

// Ingest validates the input and stores a single Metric row.
// The returned *Metric is the freshly-stored row, including
// its generated ID and timestamp. A zero Timestamp is
// replaced with the service's current time.
func (s *Service) Ingest(in IngestInput) (*Metric, error) {
	if err := validateIngest(in); err != nil {
		return nil, err
	}
	m := &Metric{
		Name:       strings.TrimSpace(in.Name),
		TargetType: in.TargetType,
		TargetID:   strings.TrimSpace(in.TargetID),
		Value:      in.Value,
		Timestamp:  in.Timestamp,
		Labels:     in.Labels,
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = s.now().UTC()
	}
	if err := s.repo.Create(m); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to ingest metric",
			Cause:   err,
		}
	}
	return m, nil
}

// List returns a page of metrics plus the unfiltered total.
// The service enforces the 90-day query-range cap before
// calling the repository.
func (s *Service) List(f ListFilter) ([]Metric, int64, error) {
	if err := validateRange(f.From, f.To); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list metrics",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// ListSeries returns the unique (name, target_type,
// target_id, labels) tuples present in the table, optionally
// filtered. Time-range validation follows the same 90-day
// cap as List.
func (s *Service) ListSeries(f ListFilter) ([]MetricSeries, error) {
	if err := validateRange(f.From, f.To); err != nil {
		return nil, err
	}
	series, err := s.repo.ListSeries(f)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list series",
			Cause:   err,
		}
	}
	return series, nil
}

// GetSeries returns the data points of a single series
// identified by the supplied key, ordered by timestamp
// ascending. A missing series returns a MetricSeries with
// an empty Points slice (not a 404 — the dashboard treats
// "no data" as a valid state).
func (s *Service) GetSeries(key SeriesKey, f ListFilter) (MetricSeries, error) {
	if err := validateSeriesKey(key); err != nil {
		return MetricSeries{}, err
	}
	if err := validateRange(f.From, f.To); err != nil {
		return MetricSeries{}, err
	}
	points, err := s.repo.GetSeriesByName(key.Name, key.TargetType, key.TargetID, f)
	if err != nil {
		return MetricSeries{}, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load series",
			Cause:   err,
		}
	}
	return MetricSeries{
		Name:       key.Name,
		TargetType: key.TargetType,
		TargetID:   key.TargetID,
		Labels:     JSONMap{},
		Points:     points,
	}, nil
}

// Scrape pulls a fresh batch of observations from the
// configured scraper for the given target and persists each
// one via Ingest. The integration is what the monitor loop
// relies on; errors from the scraper are propagated so the
// caller can retry.
func (s *Service) Scrape(ctx context.Context, targetID string) ([]Metric, error) {
	if s.scraper == nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "scraper not configured",
		}
	}
	raw, err := s.scraper.Scrape(ctx, targetID)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "scraper failed",
			Cause:   err,
		}
	}
	out := make([]Metric, 0, len(raw))
	for _, m := range raw {
		stored, err := s.Ingest(IngestInput{
			Name:       m.Name,
			TargetType: m.TargetType,
			TargetID:   m.TargetID,
			Value:      m.Value,
			Timestamp:  m.Timestamp,
			Labels:     m.Labels,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, *stored)
	}
	return out, nil
}

// validateIngest is the gatekeeper for POST /api/v1/metrics.
// Required fields: Name, TargetType (enum), TargetID.
// Timestamp is optional; Value is a free float.
func validateIngest(in IngestInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	if !TargetType(in.TargetType).Valid() {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("target_type %q is not valid", in.TargetType),
		}
	}
	if strings.TrimSpace(in.TargetID) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "target_id is required",
		}
	}
	return nil
}

// validateSeriesKey is the gatekeeper for GetSeries. Name,
// TargetType (enum), TargetID must all be non-empty.
func validateSeriesKey(k SeriesKey) error {
	if strings.TrimSpace(k.Name) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "series name is required",
		}
	}
	if !TargetType(k.TargetType).Valid() {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("series target_type %q is not valid", k.TargetType),
		}
	}
	if strings.TrimSpace(k.TargetID) == "" {
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "series target_id is required",
		}
	}
	return nil
}

// validateRange enforces the 90-day cap on a (From, To)
// pair. Open-ended ranges (one of the bounds is zero) are
// always valid — the cap only kicks in when both bounds
// are set and the difference exceeds MaxQueryRange. The
// 90-day choice is documented at the constant declaration
// (see MaxQueryRange).
func validateRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return nil
	}
	if to.Sub(from) > MaxQueryRange {
		return &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: fmt.Sprintf("time range exceeds the %v cap", MaxQueryRange),
		}
	}
	return nil
}
