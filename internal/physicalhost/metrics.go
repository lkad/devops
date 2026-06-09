package physicalhost

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// DataStatus enumerates the freshness of a Metrics snapshot. The
// cache layer renders this back to the client so the UI can
// show a "stale" badge when the prober falls behind.
const (
	DataStatusFresh      = "fresh"      // all four probes succeeded
	DataStatusStale      = "stale"      // at least one probe failed but others came through
	DataStatusUnavailable = "unavailable" // every probe failed; rendering should show "—"
)

// Metrics is the JSON-serialised snapshot the handler returns
// for GET /physical-hosts/:id/metrics. The shape mirrors the
// PRD §11.3 table (CPU/内存/磁盘/Up time) plus a wrapper for
// status and collection timestamp.
type Metrics struct {
	CPU         prober.CPUMetrics    `json:"cpu"`
	Memory      prober.MemoryMetrics `json:"memory"`
	Disk        prober.DiskResult    `json:"disk"`
	Uptime      prober.UptimeMetrics `json:"uptime"`
	DataStatus  string               `json:"data_status"`
	CollectedAt time.Time            `json:"collected_at"`
	Warnings    []string             `json:"warnings,omitempty"`
}

// MetricsCollectorConfig is the constructor input. Prober is
// required (must satisfy the prober.Prober interface); Timeout
// is a per-command ceiling that defaults to 5s.
type MetricsCollectorConfig struct {
	Prober  Prober
	Timeout time.Duration
}

// MetricsCollector drives the four SSH probes that produce a
// Metrics struct. It is deliberately tolerant: a partial
// failure is recorded in Warnings and bumps DataStatus to
// stale, instead of aborting the whole collection. Only total
// failure (all four commands error out) sets unavailable.
type MetricsCollector struct {
	prober  Prober
	timeout time.Duration
	now     func() time.Time
}

// NewMetricsCollector builds a MetricsCollector.
func NewMetricsCollector(cfg MetricsCollectorConfig) *MetricsCollector {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &MetricsCollector{prober: cfg.Prober, timeout: timeout, now: time.Now}
}

// Collect runs the four SSH commands and assembles a Metrics.
// The four commands are run in parallel (each is independent)
// so a slow disk-probe doesn't hold up a fast CPU probe.
func (c *MetricsCollector) Collect(ctx context.Context, host Host) (Metrics, error) {
	type result struct {
		name string
		out  []byte
		err  error
	}
	cmds := []string{
		"nproc",
		"top -bn1 | head -5",
		"free -m",
		"df -BG",
		"uptime -p",
		"cat /proc/uptime",
	}
	results := make(chan result, len(cmds))
	var wg sync.WaitGroup
	for _, cmd := range cmds {
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			pCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()
			out, err := c.prober.SSHExec(pCtx, host, cmd)
			results <- result{name: cmd, out: out, err: err}
		}(cmd)
	}
	wg.Wait()
	close(results)

	// Bucket results by command so the parser can use them.
	byCmd := make(map[string][]byte, len(cmds))
	var errs []string
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.name+": "+r.err.Error())
			continue
		}
		byCmd[r.name] = r.out
	}

	m := Metrics{
		Disk:        prober.DiskResult{Disks: []prober.DiskMetrics{}},
		DataStatus:  DataStatusFresh,
		CollectedAt: c.now().UTC(),
	}
	if len(errs) == len(cmds) {
		// Total failure.
		m.DataStatus = DataStatusUnavailable
		m.Warnings = errs
		return m, nil
	}
	if len(errs) > 0 {
		m.DataStatus = DataStatusStale
		m.Warnings = errs
	}

	m.CPU = prober.ParseCPU(string(byCmd["nproc"]), string(byCmd["top -bn1 | head -5"]))
	m.Memory = prober.ParseMemory(string(byCmd["free -m"]))
	if rawDF, ok := byCmd["df -BG"]; ok {
		m.Disk = prober.ParseDisk(string(rawDF))
	}
	m.Uptime = prober.ParseUptime(string(byCmd["uptime -p"]), string(byCmd["cat /proc/uptime"]))
	return m, nil
}

// renderWarningTruncated caps each warning line at 200 chars
// so a pathological SSH error doesn't bloat the JSON envelope.
// Kept private and only used in the test path.
func renderWarningTruncated(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
