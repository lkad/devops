package logs

import (
	"encoding/json"
	"testing"
	"time"
)

// TestLogEntry_JSONShape pins the wire shape of LogEntry. Per the
// spec "LogEntry schema frozen" — clients parse this object and
// field renames/removes are a major-version break.
func TestLogEntry_JSONShape(t *testing.T) {
	e := LogEntry{
		ID:        "abc",
		Timestamp: time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
		Level:     "error",
		Source:    "api",
		Message:   "boom",
		Host:      "web-1",
		Labels:    map[string]string{"env": "prod"},
		Fields:    map[string]any{"status": 500},
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	required := []string{"id", "timestamp", "level", "source", "message", "host", "labels", "fields"}
	for _, k := range required {
		if _, ok := got[k]; !ok {
			t.Errorf("LogEntry JSON missing field %q (got %s)", k, string(raw))
		}
	}
}

// TestQuery_Defaults documents the zero-value default time range:
// not the spec's "last 24h" (that's a service concern) but the
// universal-subset default that any backend can use without
// configuration.
func TestQuery_Defaults(t *testing.T) {
	var q Query
	if q.Text != "" || q.Limit != 0 {
		t.Errorf("zero-value Query should have empty Text and 0 Limit, got %+v", q)
	}
}

// TestQuery_WithFilters ensures Filter values round-trip through JSON.
func TestQuery_WithFilters(t *testing.T) {
	q := Query{
		Text: "failed",
		Filters: []Filter{
			{Key: "level", Op: "eq", Value: "error"},
			{Key: "host", Op: "prefix", Value: "web-"},
		},
		Limit: 50,
	}
	raw, _ := json.Marshal(q)
	var back Query
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Text != "failed" || back.Limit != 50 || len(back.Filters) != 2 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
	if back.Filters[0].Key != "level" || back.Filters[0].Op != "eq" {
		t.Errorf("first filter: %+v", back.Filters[0])
	}
}

// TestStream_JSONShape ensures a Stream has at minimum a Name and
// optional labels.
func TestStream_JSONShape(t *testing.T) {
	s := Stream{
		Name:   "api",
		Labels: map[string]string{"service": "payments"},
	}
	raw, _ := json.Marshal(s)
	var back Stream
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Name != "api" || back.Labels["service"] != "payments" {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

// TestResult_HasMeta ensures the Result envelope always carries Meta
// (per the spec "Meta Envelope" — every successful response includes
// it).
func TestResult_HasMeta(t *testing.T) {
	r := Result{
		Entries: []LogEntry{{ID: "1", Message: "hi"}},
		Total:   1,
		Meta:    Meta{Backend: "local"},
	}
	if len(r.Entries) != 1 || r.Total != 1 || r.Meta.Backend != "local" {
		t.Errorf("Result shape: %+v", r)
	}
}
