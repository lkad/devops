package metrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeHTTPClient is a scriptable http.RoundTripper used by the
// PrometheusScraper tests. It returns the canned response for
// the matching URL and records every call for assertions.
type fakeHTTPClient struct {
	resp *http.Response
	err  error
	got  *http.Request
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.got = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// TestFakeScraper_ReturnsScripted exercises the in-test
// scraper. The script is keyed by target id.
func TestFakeScraper_ReturnsScripted(t *testing.T) {
	ts := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	f := NewFakeScraper()
	f.Script("dev-1", []Metric{
		{Name: "cpu", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.5, Timestamp: ts},
		{Name: "mem", TargetType: string(TargetPhysicalHost), TargetID: "dev-1", Value: 0.7, Timestamp: ts},
	}, nil)

	got, err := f.Scrape(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("metrics = %d, want 2", len(got))
	}
	if got[0].Name != "cpu" {
		t.Errorf("got[0].Name = %q, want cpu", got[0].Name)
	}
}

// TestFakeScraper_DefaultEmpty: an un-scripted target returns
// an empty slice and no error.
func TestFakeScraper_DefaultEmpty(t *testing.T) {
	f := NewFakeScraper()
	got, err := f.Scrape(context.Background(), "unseen")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("metrics = %d, want 0", len(got))
	}
}

// TestFakeScraper_ScriptError: a scripted error is returned
// verbatim (so the caller can distinguish "no data" from
// "scraper broken").
func TestFakeScraper_ScriptError(t *testing.T) {
	f := NewFakeScraper()
	want := errors.New("connection refused")
	f.Script("dev-1", nil, want)

	_, err := f.Scrape(context.Background(), "dev-1")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestPrometheusScraper_ParsesTextFormat: the scraper must
// parse a Prometheus exposition body and emit one Metric per
// sample, with labels parsed as JSONMap.
func TestPrometheusScraper_ParsesTextFormat(t *testing.T) {
	body := strings.NewReader(`# HELP node_cpu_usage CPU usage
# TYPE node_cpu_usage gauge
node_cpu_usage{instance="dev-1",mode="user"} 0.42
node_cpu_usage{instance="dev-1",mode="system"} 0.13
node_memory_usage{instance="dev-1"} 0.81
`)
	hc := &fakeHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(body),
			Header:     http.Header{"Content-Type": []string{"text/plain; version=0.0.4"}},
		},
	}
	s := NewPrometheusScraper(hc, "http://example.invalid/metrics", string(TargetPhysicalHost))
	got, err := s.Scrape(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("metrics = %d, want 3", len(got))
	}
	// Verify the labels for one sample.
	var cpu *Metric
	for i := range got {
		if got[i].Name == "node_cpu_usage" {
			cpu = &got[i]
			break
		}
	}
	if cpu == nil {
		t.Fatal("node_cpu_usage not found in parsed metrics")
	}
	if cpu.Value != 0.42 {
		t.Errorf("node_cpu_usage = %v, want 0.42", cpu.Value)
	}
	if cpu.Labels["instance"] != "dev-1" {
		t.Errorf("Labels[instance] = %v, want dev-1", cpu.Labels["instance"])
	}
	if cpu.TargetType != string(TargetPhysicalHost) {
		t.Errorf("TargetType = %q, want physical_host", cpu.TargetType)
	}
	if cpu.TargetID != "dev-1" {
		t.Errorf("TargetID = %q, want dev-1", cpu.TargetID)
	}
	if cpu.Timestamp.IsZero() {
		t.Error("Timestamp was not populated")
	}
}

// TestPrometheusScraper_HTTPError: a 5xx response is an error
// (so the monitor can retry).
func TestPrometheusScraper_HTTPError(t *testing.T) {
	hc := &fakeHTTPClient{
		resp: &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	s := NewPrometheusScraper(hc, "http://example.invalid/metrics", string(TargetPhysicalHost))
	_, err := s.Scrape(context.Background(), "dev-1")
	if err == nil {
		t.Fatal("expected error on 503 response")
	}
}

// TestPrometheusScraper_TransportError: a transport error is
// propagated.
func TestPrometheusScraper_TransportError(t *testing.T) {
	hc := &fakeHTTPClient{err: errors.New("dial tcp: timeout")}
	s := NewPrometheusScraper(hc, "http://example.invalid/metrics", string(TargetPhysicalHost))
	_, err := s.Scrape(context.Background(), "dev-1")
	if err == nil {
		t.Fatal("expected error on transport failure")
	}
}

// TestPrometheusScraper_UsesContext: the request passed to
// the http client must carry the caller's context so
// cancellations propagate. A live (not yet cancelled)
// context with a deadline is the realistic case — already-
// cancelled contexts short-circuit at ctx.Err() and never
// reach the wire.
func TestPrometheusScraper_UsesContext(t *testing.T) {
	hc := &fakeHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	s := NewPrometheusScraper(hc, "http://example.invalid/metrics", string(TargetPhysicalHost))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.Scrape(ctx, "dev-1"); err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if hc.got == nil {
		t.Fatal("no request was sent")
	}
	if hc.got.Context() != ctx {
		t.Error("request did not carry the caller's context")
	}
}

// TestPrometheusScraper_SkipsCommentsAndBlankLines: the
// parser must tolerate the standard Prometheus
// "comments + HELP + TYPE + sample" layout.
func TestPrometheusScraper_SkipsCommentsAndBlankLines(t *testing.T) {
	body := strings.NewReader(`
# HELP foo Foo metric
# TYPE foo gauge
foo{label="bar"} 1.0
foo{label="baz"} 2.0
`)
	hc := &fakeHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(body),
		},
	}
	s := NewPrometheusScraper(hc, "http://example.invalid/metrics", string(TargetPhysicalHost))
	got, err := s.Scrape(context.Background(), "dev-1")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("metrics = %d, want 2", len(got))
	}
}
