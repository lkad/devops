package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHTTPClient is the seam between the ES / Loki backends and
// the network. Tests inject a script of (URL → response) pairs; the
// backend picks the matching response per call. This keeps the
// test hermetic and matches the spec rule "no real ES/Loki in
// tests".
type fakeHTTPClient struct {
	mu      sync.Mutex
	script  []fakeHTTPExchange
	calls   []fakeHTTPCall
	failErr error
}

type fakeHTTPExchange struct {
	match func(method, path string, body []byte) bool
	resp  *http.Response
}

type fakeHTTPCall struct {
	Method string
	URL    string
	Body   []byte
}

func newFakeHTTPClient() *fakeHTTPClient {
	return &fakeHTTPClient{}
}

func (c *fakeHTTPClient) addJSON(match func(string, string, []byte) bool, status int, body any) {
	raw, _ := json.Marshal(body)
	c.script = append(c.script, fakeHTTPExchange{
		match: match,
		resp: &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(string(raw))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		},
	})
}

func (c *fakeHTTPClient) addError(err error) { c.failErr = err }

func (c *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	c.calls = append(c.calls, fakeHTTPCall{
		Method: req.Method,
		URL:    req.URL.String(),
		Body:   body,
	})
	if c.failErr != nil {
		return nil, c.failErr
	}
	for _, ex := range c.script {
		if ex.match(req.Method, req.URL.Path, body) {
			return ex.resp, nil
		}
	}
	// Default: empty 404 so an un-stubbed call is loud.
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(`{"error":"not stubbed"}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (c *fakeHTTPClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

// TestES_Capabilities covers the spec matrix row: aggregation
// supported, no time-range cap, 10k max page.
func TestES_Capabilities(t *testing.T) {
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: newFakeHTTPClient()})
	c := es.Capabilities()
	if c.BackendName != "elasticsearch" {
		t.Errorf("BackendName = %q, want elasticsearch", c.BackendName)
	}
	if !c.SupportsAggregation {
		t.Errorf("ES must support aggregation")
	}
	if c.MaxTimeRange != 0 {
		t.Errorf("ES MaxTimeRange = %v, want 0 (unlimited)", c.MaxTimeRange)
	}
	if c.MaxQueryLength != 4096 {
		t.Errorf("MaxQueryLength = %d, want 4096", c.MaxQueryLength)
	}
}

// TestES_Query_UniversalSubset: a single _search call is made and
// entries are extracted from the hits.hits array.
func TestES_Query_UniversalSubset(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return m == http.MethodPost && p == "/devops-logs/_search" },
		200,
		map[string]any{
			"hits": map[string]any{
				"total": map[string]any{"value": 2},
				"hits": []map[string]any{
					{"_source": map[string]any{
						"timestamp": "2026-06-05T12:00:00Z",
						"level":     "error",
						"source":    "api",
						"message":   "boom",
						"host":      "web-1",
					}},
					{"_source": map[string]any{
						"timestamp": "2026-06-05T12:01:00Z",
						"level":     "warn",
						"source":    "worker",
						"message":   "retry",
						"host":      "worker-2",
					}},
				},
			},
		},
	)
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: fc})
	res, err := es.Query(context.Background(), Query{
		From:  time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		To:    time.Date(2026, 6, 5, 23, 59, 59, 0, time.UTC),
		Text:  "boom",
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("Total = %d, want 2", res.Total)
	}
	if len(res.Entries) != 2 {
		t.Errorf("len(Entries) = %d, want 2", len(res.Entries))
	}
	if res.Entries[0].Level != "error" {
		t.Errorf("first entry.Level = %q, want error", res.Entries[0].Level)
	}
	if res.Meta.Backend != "elasticsearch" {
		t.Errorf("Meta.Backend = %q, want elasticsearch", res.Meta.Backend)
	}
	if fc.callCount() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", fc.callCount())
	}
}

// TestES_Query_TranslatesFilters asserts the Universal DSL is
// translated to a Lucene bool query (the request body must contain
// the level filter, the substring match, and the time range).
func TestES_Query_TranslatesFilters(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return m == http.MethodPost && p == "/devops-logs/_search" },
		200,
		map[string]any{
			"hits": map[string]any{
				"total": map[string]any{"value": 0},
				"hits":  []map[string]any{},
			},
		},
	)
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: fc})
	_, err := es.Query(context.Background(), Query{
		From:    time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 6, 5, 23, 59, 59, 0, time.UTC),
		Text:    "boom",
		Filters: []Filter{{Key: "level", Op: "eq", Value: "error"}},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", fc.callCount())
	}
	call := fc.calls[0]
	if !strings.Contains(call.URL, "http://es.test:9200/devops-logs/_search") {
		t.Errorf("URL = %q, want suffix /devops-logs/_search", call.URL)
	}
	body := string(call.Body)
	if !strings.Contains(body, `"level":"error"`) {
		t.Errorf("body missing level filter: %s", body)
	}
	if !strings.Contains(body, `"message"`) {
		t.Errorf("body missing message search: %s", body)
	}
	if !strings.Contains(body, `"range"`) {
		t.Errorf("body missing time range: %s", body)
	}
}

// TestES_Query_BackendUnavailable: HTTP error is mapped to
// BACKEND_UNAVAILABLE per the spec mapping table.
func TestES_Query_BackendUnavailable(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addError(fmt.Errorf("dial tcp: i/o timeout"))
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: fc})
	_, err := es.Query(context.Background(), Query{
		From: time.Now().Add(-time.Hour), To: time.Now(), Limit: 10,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "BACKEND_UNAVAILABLE") &&
		!strings.Contains(err.Error(), "unavailable") {
		t.Errorf("error = %v, want BACKEND_UNAVAILABLE", err)
	}
}

// TestES_Streams_HitsIndices: Streams calls /_cat/indices and
// returns one Stream per index.
func TestES_Streams_HitsIndices(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return m == http.MethodGet && strings.Contains(p, "_cat/indices") },
		200,
		[]map[string]string{
			{"index": "devops-logs"},
			{"index": "devops-logs-2026.06.04"},
		},
	)
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: fc})
	streams, err := es.Streams(context.Background())
	if err != nil {
		t.Fatalf("Streams: %v", err)
	}
	if len(streams) != 2 {
		t.Errorf("len(streams) = %d, want 2 (%+v)", len(streams), streams)
	}
}

// TestES_Query_BuildsRequestWithSize: limit is translated to the
// ES "size" parameter.
func TestES_Query_BuildsRequestWithSize(t *testing.T) {
	fc := newFakeHTTPClient()
	fc.addJSON(
		func(m, p string, _ []byte) bool { return p == "/devops-logs/_search" },
		200,
		map[string]any{"hits": map[string]any{"total": map[string]any{"value": 0}, "hits": []map[string]any{}}},
	)
	es := NewES(ESConfig{BaseURL: "http://es.test:9200", HTTPClient: fc})
	_, err := es.Query(context.Background(), Query{
		From: time.Now().Add(-time.Hour), To: time.Now(), Limit: 42,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !strings.Contains(string(fc.calls[0].Body), `"size":42`) {
		t.Errorf("body missing size:42, got %s", string(fc.calls[0].Body))
	}
}
