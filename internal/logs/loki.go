package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LokiConfig is the connection-level configuration for the Loki
// backend. BaseURL is the Loki /loki/api/v1 root (no trailing
// slash). HTTPClient is the seam.
type LokiConfig struct {
	BaseURL    string
	HTTPClient HTTPClient
}

// Loki is the Loki LogBackend. The 30-day MaxTimeRange is a
// hard limit at the spec level; the service layer rejects
// requests over that with ErrTimeRangeExceeded (422). The backend
// itself only validates the soft limit.
type Loki struct {
	cfg LokiConfig
}

// NewLoki constructs a Loki backend. The HTTPClient must be set
// (use *http.Client{Timeout: 5*time.Second} in production).
func NewLoki(cfg LokiConfig) *Loki {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Loki{cfg: cfg}
}

// Capabilities returns the Loki capability row: 30d max, partial
// aggregation (count, histogram), 4096 MaxQueryLength.
func (l *Loki) Capabilities() Capabilities {
	return Capabilities{
		SupportsAggregation: true,
		MaxTimeRange:        30 * 24 * time.Hour,
		MaxQueryLength:      4096,
		BackendName:         "loki",
	}
}

// Query translates the universal Query into a LogQL query and
// hits /loki/api/v1/query_range. Streams are flattened into
// LogEntry rows.
func (l *Loki) Query(ctx context.Context, q Query) (Result, error) {
	ql := buildLogQL(q)
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	v := url.Values{}
	v.Set("query", ql)
	v.Set("start", strconv.FormatInt(q.From.UnixNano(), 10))
	if !q.To.IsZero() {
		v.Set("end", strconv.FormatInt(q.To.UnixNano(), 10))
	}
	v.Set("limit", strconv.Itoa(limit))
	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range?%s", l.cfg.BaseURL, v.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("loki request: %w", err)
	}
	resp, err := l.cfg.HTTPClient.Do(req)
	if err != nil {
		return Result{}, ErrBackendUnavailable("loki", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		buf, _ := io.ReadAll(resp.Body)
		return Result{}, ErrBackendUnavailable("loki",
			fmt.Errorf("status %d: %s", resp.StatusCode, string(buf)))
	}
	if resp.StatusCode == 400 {
		buf, _ := io.ReadAll(resp.Body)
		// Loki 400s on time-range > limit; the service layer
		// catches this before the request, but if the
		// hard-coded limit drifts we surface a 422 here.
		if strings.Contains(string(buf), "limit") {
			return Result{}, ErrTimeRangeExceeded(0, 30*24)
		}
		return Result{}, &apiError{Code: "INVALID_QUERY", Message: string(buf)}
	}
	var parsed struct {
		Data struct {
			ResultType string                          `json:"resultType"`
			Result     []map[string]any                `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{}, fmt.Errorf("loki decode: %w", err)
	}
	entries := []LogEntry{}
	for _, stream := range parsed.Data.Result {
		labels, _ := stream["stream"].(map[string]any)
		vals, _ := stream["values"].([]any)
		source, _ := labels["source"].(string)
		level, _ := labels["level"].(string)
		host, _ := labels["host"].(string)
		for _, v := range vals {
			row, _ := v.([]any)
			if len(row) < 2 {
				continue
			}
			tsStr, _ := row[0].(string)
			msg, _ := row[1].(string)
			tsNano, _ := strconv.ParseInt(tsStr, 10, 64)
			entries = append(entries, LogEntry{
				Timestamp: time.Unix(0, tsNano).UTC(),
				Level:     level,
				Source:    source,
				Host:      host,
				Message:   msg,
				Labels:    labelsToStringMap(labels),
			})
		}
	}
	return Result{
		Entries: entries,
		Total:   int64(len(entries)),
		Meta: Meta{
			Backend: "loki",
			Limits: map[string]any{
				"max_page_size":  5000,
				"max_time_range": (30 * 24 * time.Hour).String(),
			},
		},
	}, nil
}

// Streams returns the values of the "source" label as Stream rows.
func (l *Loki) Streams(ctx context.Context) ([]Stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/loki/api/v1/label/source/values", l.cfg.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("loki streams request: %w", err)
	}
	resp, err := l.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, ErrBackendUnavailable("loki", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, ErrBackendUnavailable("loki",
			fmt.Errorf("status %d", resp.StatusCode))
	}
	var parsed struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("loki streams decode: %w", err)
	}
	out := make([]Stream, 0, len(parsed.Data))
	for _, s := range parsed.Data {
		out = append(out, Stream{Name: s})
	}
	return out, nil
}

// buildLogQL translates the universal Query into a LogQL query.
// The selector uses eq filters; substring becomes a line filter
// `|~ "text"`. Unknown filter ops are skipped.
func buildLogQL(q Query) string {
	selectorParts := []string{}
	for _, f := range q.Filters {
		if f.Op != "eq" {
			continue
		}
		selectorParts = append(selectorParts, fmt.Sprintf(`%s="%s"`, f.Key, f.Value))
	}
	selector := "{" + strings.Join(selectorParts, ",") + "}"
	if q.Text == "" {
		return selector
	}
	return fmt.Sprintf(`%s |~ "%s"`, selector, q.Text)
}

// labelsToStringMap converts a Loki labels map[string]any to the
// LogEntry.Labels map[string]string. Non-string values are
// stringified so they survive the wire.
func labelsToStringMap(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprintf("%v", v)
		}
	}
	return out
}
