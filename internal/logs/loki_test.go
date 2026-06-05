package logs

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestLoki_Capabilities pins the spec matrix row: 30-day max time
// range is the hard limit, no MaxQueryLength cap, aggregation
// supported (limited).
func TestLoki_Capabilities(t *testing.T) {
	l := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: newFakeHTTPClient()})
	c := l.Capabilities()
	if c.BackendName != "loki" {
		t.Errorf("BackendName = %q, want loki", c.BackendName)
	}
	if !c.SupportsAggregation {
		t.Errorf("Loki must support aggregation")
	}
	if c.MaxTimeRange != 30*24*time.Hour {
		t.Errorf("MaxTimeRange = %v, want 30d", c.MaxTimeRange)
	}
	if c.MaxQueryLength != 4096 {
		t.Errorf("MaxQueryLength = %d, want 4096", c.MaxQueryLength)
	}
}

// TestLoki_Query_UniversalSubset hits the /loki/api/v1/query_range
// endpoint with a LogQL query and parses the response. The
// request body must include start/end and the query string.
func TestLoki_Query_UniversalSubset(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool {
			return m == http.MethodGet && strings.Contains(p, "/loki/api/v1/query_range")
		},
		200,
		map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "streams",
				"result": []map[string]any{
					{
						"stream": map[string]any{"source": "api", "level": "error"},
						"values": [][]string{
							{"1717584000000000000", "connection refused"},
						},
					},
				},
			},
		},
	)
	l := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: fc})
	res, err := l.Query(context.Background(), Query{
		From:  time.Unix(1717583900, 0).UTC(),
		To:    time.Unix(1717584100, 0).UTC(),
		Text:  "refused",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1", res.Total)
	}
	if len(res.Entries) != 1 {
		t.Errorf("len(Entries) = %d, want 1", len(res.Entries))
	}
	if res.Entries[0].Source != "api" {
		t.Errorf("entry.Source = %q, want api", res.Entries[0].Source)
	}
	if res.Entries[0].Message != "connection refused" {
		t.Errorf("entry.Message = %q, want 'connection refused'", res.Entries[0].Message)
	}
	if res.Meta.Backend != "loki" {
		t.Errorf("Meta.Backend = %q, want loki", res.Meta.Backend)
	}
	if fc.callCount() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", fc.callCount())
	}
}

// TestLoki_Query_LogQLTranslation checks the LogQL string is
// built correctly: the substring search becomes `|~ "..."` and
// filter terms become `{level="error"}`.
func TestLoki_Query_LogQLTranslation(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return strings.Contains(p, "query_range") },
		200,
		map[string]any{
			"data": map[string]any{
				"resultType": "streams",
				"result":     []map[string]any{},
			},
		},
	)
	l := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: fc})
	_, err := l.Query(context.Background(), Query{
		From:    time.Unix(1717583900, 0).UTC(),
		To:      time.Unix(1717584100, 0).UTC(),
		Text:    "boom",
		Filters: []Filter{{Key: "level", Op: "eq", Value: "error"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	url := fc.calls[0].URL
	if !strings.Contains(url, `query=%7Blevel%3D%22error%22%7D`) {
		t.Errorf("URL missing label selector: %s", url)
	}
	if !strings.Contains(url, "%7C~+%22boom%22") && !strings.Contains(url, "%7C~%20%22boom%22") {
		t.Errorf("URL missing substring filter: %s", url)
	}
}

// TestLoki_Query_TimeRange30d_Accepted: a 30-day range is the
// hard limit, accepted.
func TestLoki_Query_TimeRange30d_Accepted(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return strings.Contains(p, "query_range") },
		200,
		map[string]any{
			"data": map[string]any{
				"resultType": "streams",
				"result":     []map[string]any{},
			},
		},
	)
	l := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: fc})
	_, err := l.Query(context.Background(), Query{
		From:  time.Now().Add(-30 * 24 * time.Hour),
		To:    time.Now(),
		Limit: 10,
	})
	if err != nil {
		t.Errorf("30d range should be accepted, got %v", err)
	}
}

// TestLoki_Streams lists known labels as streams.
func TestLoki_Streams(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return strings.Contains(p, "/loki/api/v1/label") },
		200,
		map[string]any{
			"data": []string{"api", "worker", "database"},
		},
	)
	l := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: fc})
	streams, err := l.Streams(context.Background())
	if err != nil {
		t.Fatalf("Streams: %v", err)
	}
	if len(streams) != 3 {
		t.Errorf("len(streams) = %d, want 3", len(streams))
	}
}
