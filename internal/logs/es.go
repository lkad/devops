package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient is the seam between the network-bound backends (ES,
// Loki) and the rest of the package. Tests inject fakeHTTPClient;
// production uses *http.Client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ESConfig is the connection-level configuration for the
// Elasticsearch backend. Index is the log index name (default
// "devops-logs"). HTTPClient is the seam — pass *http.Client in
// production, fakeHTTPClient in tests.
type ESConfig struct {
	BaseURL    string
	Index      string
	HTTPClient HTTPClient
}

// ES is the Elasticsearch LogBackend. The HTTPClient is the only
// mutable field; everything else is read-only after construction.
type ES struct {
	cfg ESConfig
}

// NewES constructs an ES backend. The HTTPClient must be set (use
// &http.Client{Timeout: 5*time.Second} in production).
func NewES(cfg ESConfig) *ES {
	if cfg.Index == "" {
		cfg.Index = "devops-logs"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &ES{cfg: cfg}
}

// Capabilities returns the ES capability row: aggregation
// supported, no time-range cap, 4096 max query length.
func (e *ES) Capabilities() Capabilities {
	return Capabilities{
		SupportsAggregation: true,
		MaxTimeRange:        0, // unlimited
		MaxQueryLength:      4096,
		BackendName:         "elasticsearch",
	}
}

// Query translates the universal DSL into a Lucene bool query and
// POSTs it to /<index>/_search. The response's hits.hits are
// mapped to LogEntry rows.
func (e *ES) Query(ctx context.Context, q Query) (Result, error) {
	body := e.buildRequest(q)
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("es marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/%s/_search", e.cfg.BaseURL, e.cfg.Index), bytes.NewReader(raw))
	if err != nil {
		return Result{}, fmt.Errorf("es request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.cfg.HTTPClient.Do(req)
	if err != nil {
		return Result{}, ErrBackendUnavailable("elasticsearch", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		buf, _ := io.ReadAll(resp.Body)
		return Result{}, ErrBackendUnavailable("elasticsearch",
			fmt.Errorf("status %d: %s", resp.StatusCode, string(buf)))
	}
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		return Result{}, &apiError{
			Code:    "INVALID_QUERY",
			Message: fmt.Sprintf("es query rejected: %s", string(buf)),
		}
	}
	var parsed esResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Result{}, fmt.Errorf("es decode: %w", err)
	}
	entries := make([]LogEntry, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		entries = append(entries, h.Source.toLogEntry())
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	return Result{
		Entries: entries,
		Total:   int64(parsed.Hits.Total.Value),
		Meta: Meta{
			Backend: "elasticsearch",
			Limits: map[string]any{
				"max_page_size": 10000,
			},
		},
	}, nil
}

// Streams calls GET /_cat/indices?h=index and returns one Stream
// per index that starts with the configured index name.
func (e *ES) Streams(ctx context.Context) ([]Stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/_cat/indices/%s*?h=index&format=json", e.cfg.BaseURL, e.cfg.Index), nil)
	if err != nil {
		return nil, fmt.Errorf("es streams request: %w", err)
	}
	resp, err := e.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, ErrBackendUnavailable("elasticsearch", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, ErrBackendUnavailable("elasticsearch",
			fmt.Errorf("status %d", resp.StatusCode))
	}
	var rows []map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("es streams decode: %w", err)
	}
	out := make([]Stream, 0, len(rows))
	for _, r := range rows {
		name := r["index"]
		if name == "" {
			continue
		}
		out = append(out, Stream{Name: name})
	}
	return out, nil
}

// buildRequest translates the universal Query into the Lucene bool
// query body. We keep this hand-written (no SDK) so the wire shape
// is auditable in tests.
func (e *ES) buildRequest(q Query) map[string]any {
	must := []map[string]any{}
	// Substring search → match on message.
	if strings.TrimSpace(q.Text) != "" {
		must = append(must, map[string]any{
			"match": map[string]any{
				"message": q.Text,
			},
		})
	}
	// Filters → term queries.
	for _, f := range q.Filters {
		if f.Op != "eq" {
			continue
		}
		must = append(must, map[string]any{
			"term": map[string]any{
				f.Key: f.Value,
			},
		})
	}
	// Time range → range query.
	if !q.From.IsZero() || !q.To.IsZero() {
		r := map[string]any{}
		if !q.From.IsZero() {
			r["gte"] = q.From.Format(time.RFC3339)
		}
		if !q.To.IsZero() {
			r["lte"] = q.To.Format(time.RFC3339)
		}
		must = append(must, map[string]any{
			"range": map[string]any{
				"timestamp": r,
			},
		})
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	body := map[string]any{
		"size":  limit,
		"sort":  []map[string]any{{"timestamp": map[string]any{"order": sortOrder(q.Sort)}}},
		"query": map[string]any{"bool": map[string]any{"must": must}},
	}
	return body
}

// sortOrder maps the universal "time:asc" / "time:desc" sort to
// the ES "asc" / "desc" string. Empty defaults to "desc".
func sortOrder(s string) string {
	if strings.HasSuffix(s, ":asc") {
		return "asc"
	}
	return "desc"
}

// esResponse is the minimal subset of the ES search response we
// need. Field names are exact (ES is case-sensitive on JSON).
type esResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source esSource `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// esSource mirrors the doc shape we POST to ES. We accept the
// common fields; unknown fields are ignored.
type esSource struct {
	Timestamp string            `json:"timestamp"`
	Level     string            `json:"level"`
	Source    string            `json:"source"`
	Message   string            `json:"message"`
	Host      string            `json:"host"`
	Labels    map[string]string `json:"labels"`
	Fields    map[string]any    `json:"fields"`
}

func (s esSource) toLogEntry() LogEntry {
	ts, _ := time.Parse(time.RFC3339, s.Timestamp)
	return LogEntry{
		Timestamp: ts,
		Level:     s.Level,
		Source:    s.Source,
		Message:   s.Message,
		Host:      s.Host,
		Labels:    s.Labels,
		Fields:    s.Fields,
	}
}

// apiError is a minimal error type for client-side query errors.
// We avoid importing contracts here to keep the package boundary
// thin; the service layer maps these to APIError.
type apiError struct {
	Code    string
	Message string
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }
