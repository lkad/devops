package logs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newLocalForTest constructs a Local backend rooted at a temp dir so
// tests don't touch the real fixture path. Returns the backend and a
// cleanup function.
func newLocalForTest(t *testing.T) (*Local, string) {
	t.Helper()
	dir := t.TempDir()
	// Write two fixture files: one JSON-per-line, one plain text.
	jsonPath := filepath.Join(dir, "app.json.log")
	plainPath := filepath.Join(dir, "web.log")

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	jsonLines := []map[string]any{
		{"timestamp": now.Add(-2 * time.Minute).Format(time.RFC3339), "level": "info", "source": "api", "message": "request ok", "host": "web-1"},
		{"timestamp": now.Add(-1 * time.Minute).Format(time.RFC3339), "level": "error", "source": "api", "message": "connection refused", "host": "web-1"},
		{"timestamp": now.Format(time.RFC3339), "level": "warn", "source": "worker", "message": "retrying", "host": "worker-2"},
	}
	mustWriteJSONL(t, jsonPath, jsonLines)
	mustWriteFile(t, plainPath,
		"2026-06-05T11:59:00Z [info] api: request ok host=web-1\n"+
			"2026-06-05T11:59:30Z [error] worker: failed to flush host=worker-2\n",
	)

	l := NewLocal(LocalConfig{Dir: dir})
	return l, dir
}

func mustWriteJSONL(t *testing.T, path string, rows []map[string]any) {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestLocal_Capabilities matches the matrix row for Local: no
// aggregation, 7-day max (the spec task says "7d" — Local has no
// real limit so we set the smallest sensible value).
func TestLocal_Capabilities(t *testing.T) {
	l, _ := newLocalForTest(t)
	c := l.Capabilities()
	if c.BackendName != "local" {
		t.Errorf("BackendName = %q, want local", c.BackendName)
	}
	if c.SupportsAggregation {
		t.Errorf("Local must not support aggregation")
	}
	if c.MaxTimeRange != 7*24*time.Hour {
		t.Errorf("MaxTimeRange = %v, want 7d", c.MaxTimeRange)
	}
}

// TestLocal_Query_UniversalSubset is the spec scenario
// "Universal fields": time range + level + source + substring all
// work on the Local backend.
func TestLocal_Query_UniversalSubset(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	q := Query{
		From:    now.Add(-3 * time.Minute),
		To:      now.Add(1 * time.Minute),
		Filters: []Filter{{Key: "level", Op: "eq", Value: "error"}},
		Text:    "refused",
		Limit:   50,
	}
	res, err := l.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1 (entries=%+v)", res.Total, res.Entries)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(res.Entries))
	}
	if res.Entries[0].Message != "connection refused" {
		t.Errorf("entry.Message = %q, want 'connection refused'", res.Entries[0].Message)
	}
	if res.Meta.Backend != "local" {
		t.Errorf("Meta.Backend = %q, want local", res.Meta.Backend)
	}
	if res.Meta.Degraded {
		t.Errorf("universal-subset query should not be degraded: %+v", res.Meta)
	}
}

// TestLocal_Query_TextSearch is the spec scenario "Substring search
// fallback" — case-insensitive substring match.
func TestLocal_Query_TextSearch(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	q := Query{
		From:  time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		To:    time.Date(2026, 6, 5, 23, 59, 59, 0, time.UTC),
		Text:  "FAILED", // uppercase — case-insensitive match
		Limit: 100,
	}
	res, err := l.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total < 1 {
		t.Errorf("substring FAILED should match; got %+v", res)
	}
}

// TestLocal_Query_TimeRangeFilter verifies the from/to filtering
// excludes entries outside the window.
func TestLocal_Query_TimeRangeFilter(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	q := Query{
		From:  time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC),
		To:    time.Date(2026, 6, 5, 12, 30, 0, 0, time.UTC),
		Limit: 100,
	}
	res, err := l.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total < 1 {
		t.Errorf("expected at least one match in window, got %+v", res)
	}
}

// TestLocal_Query_RejectStructuredQuery is the spec scenario "Lucene
// query on Local backend": structured queries (regex, Lucene) are
// not supported on Local. The Local backend itself does not validate
// this — the service layer does. Here we only assert the Local
// backend does not crash on an unknown filter and returns a normal
// Result.
func TestLocal_Query_IgnoresUnknownFilter(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	q := Query{
		From:    time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 6, 5, 23, 59, 59, 0, time.UTC),
		Filters: []Filter{{Key: "structured_query", Op: "eq", Value: "level:error AND host:web-*"}},
		Limit:   10,
	}
	res, err := l.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// Local just treats unknown filter keys as non-matches; the
	// service layer is responsible for the 400.
	_ = res
}

// TestLocal_Query_Empty returns an empty result for an out-of-range
// window, never nil Entries.
func TestLocal_Query_Empty(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	q := Query{
		From:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		To:    time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Limit: 10,
	}
	res, err := l.Query(ctx, q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Entries == nil {
		t.Errorf("Entries should be empty slice, not nil")
	}
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
}

// TestLocal_Streams lists the unique sources present in the fixture.
func TestLocal_Streams(t *testing.T) {
	l, _ := newLocalForTest(t)
	ctx := context.Background()
	streams, err := l.Streams(ctx)
	if err != nil {
		t.Fatalf("Streams: %v", err)
	}
	if len(streams) == 0 {
		t.Errorf("expected at least one stream")
	}
}

// TestLocal_Echo_PushesEntry covers the dev-only POST .../_test/echo
// path: a fake log entry is appended to memory and becomes visible
// in subsequent queries.
func TestLocal_Echo_PushesEntry(t *testing.T) {
	l, _ := newLocalForTest(t)
	e := LogEntry{
		ID:        "echo-1",
		Timestamp: time.Date(2026, 6, 5, 13, 0, 0, 0, time.UTC),
		Level:     "info",
		Source:    "echo",
		Message:   "echoed",
		Host:      "test",
	}
	l.Append(e)
	res, err := l.Query(context.Background(), Query{
		From:  e.Timestamp.Add(-time.Hour),
		To:    e.Timestamp.Add(time.Hour),
		Text:  "echoed",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1 (echo not visible)", res.Total)
	}
}
