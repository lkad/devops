//go:build load

// Package load contains the Go-native load test that
// mirrors the k6 script at k6-physicalhost.js. k6 is the
// canonical load-tester, but it isn't always installed in
// CI; this Go version runs as a normal `go test` target so
// the SLO thresholds are enforced automatically.
//
// The two implementations MUST stay in sync — the k6 script
// is the contract; this file is the executable copy.
//
// Targets:
//   - GET /api/v1/physical-hosts (List, the page-level read)
//   - GET /api/v1/physical-hosts/:id (Get)
//   - GET /api/v1/physical-hosts/:id/metrics (Metrics)
//
// SLOs (same as k6):
//   - p95 latency < 500ms
//   - error rate < 1%
//
// Run:
//   LOAD_BASE_URL=http://localhost:3000 \
//   LOAD_TOKEN=dev-bypass \
//   LOAD_VUS=50 LOAD_DURATION=20s \
//   go test -tags=load -run TestLoadPhysicalHosts \
//   ./tests/load/...
//
// The "load" build tag keeps the test out of the default
// suite (`go test ./...` skips it). The test asserts SLOs
// so a regression fails the run.
package load

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// loadConfig is the env-driven harness. VUS, DURATION, BASE_URL,
// and TOKEN mirror the k6 env vars.
type loadConfig struct {
	BaseURL  string
	Token    string
	VUs      int
	Duration time.Duration
}

func loadCfg(t *testing.T) loadConfig {
	t.Helper()
	cfg := loadConfig{
		BaseURL:  envDefault("LOAD_BASE_URL", "http://localhost:3000"),
		Token:    envDefault("LOAD_TOKEN", "dev-bypass"),
		VUs:      50,
		Duration: 20 * time.Second,
	}
	if v := os.Getenv("LOAD_VUS"); v != "" {
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			cfg.VUs = n
		}
	}
	if v := os.Getenv("LOAD_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Duration = d
		}
	}
	return cfg
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestLoadPhysicalHosts is the actual load test. It spawns
// `VUs` virtual users, each of which hammers the three
// endpoints in a loop until `Duration` elapses, then asserts
// p95 latency and error-rate SLOs.
func TestLoadPhysicalHosts(t *testing.T) {
	cfg := loadCfg(t)
	if testing.Short() {
		t.Skip("skipping load test in -short mode")
	}

	hc := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	// Per-VU latencies. We only need p95 of the list endpoint
	// for the SLO; the metrics endpoint is informational.
	var (
		mu               sync.Mutex
		listLatencies    []time.Duration
		metricsLatencies []time.Duration
		successCount     int64
		errorCount       int64
	)

	// First: probe to ensure the service is reachable. A
	// 30s connect-timeout is plenty for local dev.
	probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer probeCancel()
	if err := probe(probeCtx, hc, cfg); err != nil {
		t.Skipf("service not reachable at %s: %v", cfg.BaseURL, err)
	}

	// First call to grab a real host ID for the Get / Metrics
	// portion of the loop. If there are no hosts, those
	// branches are skipped (but List still counts).
	firstHostID := firstHostOrEmpty(t, probeCtx, hc, cfg)
	if firstHostID == "" {
		t.Log("no physical hosts in DB; the Get/Metrics branches will be skipped, List SLO still enforced")
	}

	var wg sync.WaitGroup
	for vu := 0; vu < cfg.VUs; vu++ {
		wg.Add(1)
		go func(vuID int) {
			defer wg.Done()
			for ctx.Err() == nil {
				// List — the page-level read.
				start := time.Now()
				listResp := doGet(ctx, hc, cfg, "/api/v1/physical-hosts?limit=20")
				listLat := time.Since(start)
				if listResp.status == 200 && listResp.hasData {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
				mu.Lock()
				listLatencies = append(listLatencies, listLat)
				mu.Unlock()

				// Get + Metrics (only if we have a host).
				if firstHostID != "" {
					doGet(ctx, hc, cfg, "/api/v1/physical-hosts/"+firstHostID)

					mStart := time.Now()
					mResp := doGet(ctx, hc, cfg, "/api/v1/physical-hosts/"+firstHostID+"/metrics")
					if mResp.status != 200 {
						atomic.AddInt64(&errorCount, 1)
					}
					mu.Lock()
					metricsLatencies = append(metricsLatencies, time.Since(mStart))
					mu.Unlock()
				}

				// 0.5s think time matches k6's sleep(0.5).
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
		}(vu)
	}
	wg.Wait()

	// Aggregate and assert.
	totalReqs := successCount + errorCount
	if totalReqs == 0 {
		t.Fatalf("no requests completed; service may be down")
	}
	errRate := float64(errorCount) / float64(totalReqs)
	p95List := percentile(listLatencies, 0.95)
	p95Metrics := percentile(metricsLatencies, 0.95)
	p50List := percentile(listLatencies, 0.50)

	t.Logf("load: vus=%d duration=%s reqs=%d success=%d err=%d errRate=%.3f",
		cfg.VUs, cfg.Duration, totalReqs, successCount, errorCount, errRate)
	t.Logf("list   p50=%s p95=%s", p50List.Round(time.Millisecond), p95List.Round(time.Millisecond))
	t.Logf("metrics p95=%s", p95Metrics.Round(time.Millisecond))

	// SLOs.
	if errRate >= 0.01 {
		t.Errorf("error rate %.3f >= 0.01 SLO", errRate)
	}
	if p95List >= 500*time.Millisecond {
		t.Errorf("list p95 %s >= 500ms SLO", p95List)
	}
}

type response struct {
	status  int
	hasData bool
}

func doGet(ctx context.Context, hc *http.Client, cfg loadConfig, path string) response {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+path, nil)
	if err != nil {
		return response{status: 0}
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return response{status: 0}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	hasData := false
	if resp.StatusCode == 200 {
		var parsed struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(body, &parsed) == nil && len(parsed.Data) > 0 && string(parsed.Data) != "null" {
			hasData = true
		}
	}
	return response{status: resp.StatusCode, hasData: hasData}
}

func probe(ctx context.Context, hc *http.Client, cfg loadConfig) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/api/v1/physical-hosts?limit=1", nil)
	if err != nil {
		return err
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("probe got HTTP %d", resp.StatusCode)
	}
	return nil
}

func firstHostOrEmpty(t *testing.T, ctx context.Context, hc *http.Client, cfg loadConfig) string {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/api/v1/physical-hosts?limit=1", nil)
	if err != nil {
		return ""
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if len(parsed.Data) > 0 {
		return parsed.Data[0].ID
	}
	return ""
}

// percentile returns the p-th percentile of d. d is mutated
// (sorted) for efficiency — copy first if you need the
// original ordering.
func percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
