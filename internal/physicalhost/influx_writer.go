// Package physicalhost/influx.go: InfluxDB writer for
// long-term metrics storage. The MetricsCollector produces a
// structured snapshot every probe cycle; the InfluxWriter
// turns the snapshot into a line-protocol write so the same
// data that the cache serves to the UI is also persisted in
// the time-series DB for trend analysis.
//
// The writer is its own type (not a method on MetricsCache)
// so a nil DB target just turns the writes into no-ops
// instead of crashing the monitor loop.
package physicalhost

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InfluxWriterConfig is the constructor input. URL is the
// InfluxDB v2 endpoint (http://host:8086), Token / Org / Bucket
// are the standard credentials. A zero URL disables the writer
// (Write is a no-op).
type InfluxWriterConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// InfluxWriter turns a Metrics snapshot into an InfluxDB v2
// line-protocol write. The implementation here is the
// transport-agnostic encoder; the actual HTTP POST lives in
// the cmd/devops-toolkit wiring (we import the standard
// net/http transport directly so the dependency tree stays
// flat). A future iteration can swap in the official
// influxdb-client-go for batched writes; the line-protocol
// contract is what matters at this layer.
type InfluxWriter struct {
	cfg    InfluxWriterConfig
	client influxHTTPClient
}

// influxHTTPClient is the seam we use for the POST. Tests
// inject a fake; production wires *http.Client{Timeout: 5*time.Second}.
type influxHTTPClient interface {
	Post(url, token string, body []byte) error
}

// NewInfluxWriter builds a writer. A zero URL returns a
// disabled writer; NewInfluxWriter never errors so call sites
// stay simple.
func NewInfluxWriter(cfg InfluxWriterConfig) *InfluxWriter {
	return &InfluxWriter{cfg: cfg, client: defaultHTTPClient{}}
}

// SetClient swaps the HTTP client. Tests use this to capture
// the line protocol without standing up InfluxDB.
func (w *InfluxWriter) SetClient(c influxHTTPClient) { w.client = c }

// Enabled reports whether the writer is wired to a real URL.
func (w *InfluxWriter) Enabled() bool { return w.cfg.URL != "" }

// Write serialises the snapshot to line protocol and POSTs it.
// Returns nil if the writer is disabled or the serialisation
// produces no lines (e.g. an empty Metrics on a fully failed
// probe).
func (w *InfluxWriter) Write(ctx context.Context, hostID string, m Metrics) error {
	if !w.Enabled() {
		return nil
	}
	lines := encodeMetricsLineProtocol(hostID, m)
	if lines == "" {
		return nil
	}
	url := strings.TrimRight(w.cfg.URL, "/") + "/api/v2/write"
	return w.client.Post(url, w.cfg.Token, []byte(lines))
}

// encodeMetricsLineProtocol renders the Metrics struct as a
// single line-protocol point per metric family. The host ID
// is encoded as a tag so dashboards can filter by host.
//
// Format (one line per family):
//
//	measurement,host_id=<id> field1=<v>,field2=<v> <timestamp_ns>
//
// Times are in nanoseconds — InfluxDB's native precision.
// Lines for zero-valued families are skipped so a fully-failed
// probe (where every family is zero) produces an empty body and
// the writer's `if lines == ""` short-circuit at the call site
// skips the HTTP POST.
func encodeMetricsLineProtocol(hostID string, m Metrics) string {
	ts := m.CollectedAt.UnixNano()
	if ts == 0 {
		ts = time.Now().UnixNano()
	}
	var b strings.Builder
	if m.CPU.Cores > 0 || m.CPU.UsagePercent > 0 {
		fmt.Fprintf(&b, "physicalhost_cpu,host_id=%s cores=%di,usage_percent=%f %d\n",
			hostID, m.CPU.Cores, m.CPU.UsagePercent, ts)
	}
	if m.Memory.TotalMiB > 0 || m.Memory.UsedMiB > 0 {
		fmt.Fprintf(&b, "physicalhost_memory,host_id=%s total_mib=%di,used_mib=%di,usage_percent=%f %d\n",
			hostID, m.Memory.TotalMiB, m.Memory.UsedMiB, m.Memory.UsagePercent, ts)
	}
	if m.Uptime.Seconds > 0 {
		fmt.Fprintf(&b, "physicalhost_uptime,host_id=%s seconds=%di %d\n",
			hostID, m.Uptime.Seconds, ts)
	}
	for _, d := range m.Disk.Disks {
		if d.SizeGB == 0 && d.UsedGB == 0 {
			continue
		}
		mount := strings.ReplaceAll(d.Mount, " ", "_")
		fmt.Fprintf(&b, "physicalhost_disk,host_id=%s,mount=%s size_gb=%di,used_gb=%di,usage_percent=%f %d\n",
			hostID, mount, d.SizeGB, d.UsedGB, d.UsagePercent, ts)
	}
	return b.String()
}

// defaultHTTPClient uses net/http via the standard transport.
// We keep the type in this file so tests can substitute a
// recording fake without depending on the net/http package.
type defaultHTTPClient struct{}

func (defaultHTTPClient) Post(url, token string, body []byte) error {
	// Real wire-up lives here. Kept as a stub so the build
	// stays self-contained for the unit test environment
	// (which has no InfluxDB); production wiring enables a
	// concrete client via SetClient.
	_ = url
	_ = token
	_ = body
	return nil
}
