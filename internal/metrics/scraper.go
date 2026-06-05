package metrics

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HTTPDoer is the minimal contract a Scraper needs from an
// HTTP client. The standard *http.Client satisfies it; tests
// pass a fake. Defining the seam here lets the Prometheus
// scraper be unit-tested without a network round-trip.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Scraper is the interface the metrics service depends on.
// Implementations fetch raw observations from a target and
// return them as a slice of Metric rows. The Metric slice is
// in the order the scraper produced them — the caller is
// responsible for ordering / dedup / storage.
//
// All implementations must honour ctx cancellation and
// return early with a non-nil error when the deadline is
// exceeded.
type Scraper interface {
	Scrape(ctx context.Context, targetID string) ([]Metric, error)
}

// FakeScraper is the in-test Scraper. It does not perform
// any network IO; tests script behaviour by (targetID ->
// []Metric + error). The default behaviour for an
// un-scripted target is "no metrics, no error".
//
// FakeScraper is safe for concurrent use.
type FakeScraper struct {
	mu      sync.Mutex
	scripts map[string]scripted
}

type scripted struct {
	metrics []Metric
	err     error
}

// NewFakeScraper returns an empty FakeScraper. Call
// FakeScraper.Script to wire a target's response.
func NewFakeScraper() *FakeScraper {
	return &FakeScraper{scripts: map[string]scripted{}}
}

// Script pre-programs a Scrape result for one target. A nil
// err means "scrape succeeded, here are the metrics". A
// non-nil err means "scrape failed, ignore the metrics".
func (f *FakeScraper) Script(targetID string, metrics []Metric, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[targetID] = scripted{metrics: metrics, err: err}
}

// Scrape implements Scraper. Returns the scripted
// (metrics, err) pair, or (nil, nil) for an un-scripted
// target.
func (f *FakeScraper) Scrape(ctx context.Context, targetID string) ([]Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	s, ok := f.scripts[targetID]
	f.mu.Unlock()
	if !ok {
		return nil, nil
	}
	return s.metrics, s.err
}

// Compile-time check that *FakeScraper implements Scraper.
var _ Scraper = (*FakeScraper)(nil)

// PrometheusScraper pulls a Prometheus text-format
// /metrics endpoint and converts the samples into Metric
// rows. It is intentionally a stub: a minimal parser
// supporting the (line starting with a non-# metric name +
// labels + value) shape, which is what every exporter
// produces. The full Prometheus parser is out of scope for
// this phase — the test surface pins the behaviour we need
// from the rest of the system.
type PrometheusScraper struct {
	// client is the HTTP client the scraper uses. Standard
	// *http.Client satisfies HTTPDoer; tests inject a fake.
	client HTTPDoer
	// endpoint is the URL the scraper GETs on every call.
	// The targetID is appended as a query string so the
	// server can distinguish per-target scrapes.
	endpoint string
	// targetType is the TargetType the scraper stamps on
	// every Metric it produces (physical_host, k8s_pod).
	targetType string
}

// NewPrometheusScraper constructs a PrometheusScraper. The
// endpoint is the base URL; targetID is passed as the
// "target" query string. targetType is the TargetType the
// scraper stamps on every Metric it produces.
func NewPrometheusScraper(client HTTPDoer, endpoint, targetType string) *PrometheusScraper {
	return &PrometheusScraper{
		client:     client,
		endpoint:   endpoint,
		targetType: targetType,
	}
}

// Scrape implements Scraper. It GETs the configured endpoint
// with the targetID as a query string, parses the response
// body, and returns one Metric per sample. Comments and
// blank lines are ignored. A non-2xx response is an error.
func (p *PrometheusScraper) Scrape(ctx context.Context, targetID string) ([]Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sep := "?"
	if strings.Contains(p.endpoint, "?") {
		sep = "&"
	}
	url := p.endpoint + sep + "target=" + targetID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("metrics.PrometheusScraper: build request: %w", err)
	}
	req.Header.Set("Accept", "text/plain;version=0.0.4")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metrics.PrometheusScraper: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metrics.PrometheusScraper: unexpected status %d", resp.StatusCode)
	}
	return parsePrometheusText(resp.Body, targetID, p.targetType)
}

// parsePrometheusText parses the body of a Prometheus
// /metrics response into Metric rows. The format is:
//
//	# HELP <name> <text>
//	# TYPE <name> <type>
//	<name>{<label>="<value>"} <number>
//
// HELP and TYPE lines are ignored; only sample lines
// produce Metric rows. Malformed lines are skipped
// silently — a real Prometheus deployment mixes HELP /
// TYPE into the same file and a strict parser would
// reject the whole file on the first odd sample.
func parsePrometheusText(r io.Reader, targetID, targetType string) ([]Metric, error) {
	out := []Metric{}
	ts := time.Now().UTC()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, val, ok := splitSample(line)
		if !ok {
			continue
		}
		out = append(out, Metric{
			Name:       name,
			TargetType: targetType,
			TargetID:   targetID,
			Value:      val,
			Timestamp:  ts,
			Labels:     labels,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("metrics.parsePrometheusText: %w", err)
	}
	return out, nil
}

// splitSample parses "name{labels} value" into its
// components. labels is a JSONMap; the inner label set
// becomes the Metric's Labels field. Returns ok=false
// for blank / comment / unparseable lines.
func splitSample(line string) (string, JSONMap, float64, bool) {
	// Split into the "head" (name + labels) and the
	// trailing value. The value is always the last
	// whitespace-separated token.
	idx := strings.LastIndex(line, " ")
	if idx <= 0 {
		return "", nil, 0, false
	}
	head := line[:idx]
	valStr := line[idx+1:]
	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return "", nil, 0, false
	}
	name, labels := splitNameAndLabels(head)
	if name == "" {
		return "", nil, 0, false
	}
	return name, labels, v, true
}

// splitNameAndLabels parses "name" or "name{k=\"v\",...}".
// Returns an empty JSONMap when no labels are present.
func splitNameAndLabels(head string) (string, JSONMap) {
	open := strings.Index(head, "{")
	if open < 0 {
		return head, JSONMap{}
	}
	name := head[:open]
	body := head[open+1:]
	close := strings.LastIndex(body, "}")
	if close < 0 {
		// Malformed: no closing brace. Treat the
		// head as a plain name.
		return name, JSONMap{}
	}
	body = body[:close]
	labels := parseLabels(body)
	return name, labels
}

// parseLabels parses the body of a Prometheus label set
// ("k1=\"v1\",k2=\"v2\"") into a JSONMap. The parser is
// minimal: it handles the standard escaped-quote case
// (\" -> ") and ignores everything else. Good enough for
// the metrics this app emits.
func parseLabels(body string) JSONMap {
	out := JSONMap{}
	i := 0
	for i < len(body) {
		// Skip leading whitespace + comma.
		for i < len(body) && (body[i] == ' ' || body[i] == ',') {
			i++
		}
		// Read the key up to '='.
		keyStart := i
		for i < len(body) && body[i] != '=' {
			i++
		}
		if i >= len(body) {
			break
		}
		key := strings.TrimSpace(body[keyStart:i])
		i++ // skip '='
		if i >= len(body) || body[i] != '"' {
			break
		}
		i++ // skip opening quote
		// Read the value, respecting escaped quotes.
		var sb strings.Builder
		for i < len(body) && body[i] != '"' {
			if body[i] == '\\' && i+1 < len(body) && body[i+1] == '"' {
				sb.WriteByte('"')
				i += 2
				continue
			}
			sb.WriteByte(body[i])
			i++
		}
		if i < len(body) {
			i++ // skip closing quote
		}
		out[key] = sb.String()
	}
	return out
}

// Compile-time check that *PrometheusScraper implements
// Scraper.
var _ Scraper = (*PrometheusScraper)(nil)
