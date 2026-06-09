package physicalhost

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// recordingClient captures the body the writer sends. The
// optional counter / mu / blocking fields are used by the
// async-writer test to race the worker.
type recordingClient struct {
	mu       *sync.Mutex
	body     []byte
	counter  *atomic.Int32
	blocking chan struct{}
}

func (r *recordingClient) Post(_, _ string, body []byte) error {
	if r.mu != nil {
		r.mu.Lock()
	}
	r.body = body
	if r.mu != nil {
		r.mu.Unlock()
	}
	if r.counter != nil {
		// Block until the test releases us. Lets the overflow
		// test build pressure in the channel without racing.
		if r.blocking != nil {
			<-r.blocking
		}
		r.counter.Add(1)
	}
	return nil
}

// TestInfluxWriter_DisabledByDefault confirms a zero-URL
// writer is a no-op (no panic, no error).
func TestInfluxWriter_DisabledByDefault(t *testing.T) {
	w := NewInfluxWriter(InfluxWriterConfig{})
	if w.Enabled() {
		t.Error("zero-URL writer should be disabled")
	}
	if err := w.Write(context.Background(), "h-1", Metrics{}); err != nil {
		t.Errorf("disabled writer should not error: %v", err)
	}
}

// TestInfluxWriter_EncodesAllFields checks the line protocol
// output: one line per family, with the host_id tag, fields
// in scientific or integer form, and a nanosecond timestamp.
func TestInfluxWriter_EncodesAllFields(t *testing.T) {
	rec := &recordingClient{}
	w := NewInfluxWriter(InfluxWriterConfig{
		URL: "http://influx:8086", Token: "t", Org: "devops", Bucket: "metrics",
	})
	w.SetClient(rec)

	m := Metrics{
		CPU:        prober.CPUMetrics{Cores: 8, UsagePercent: 12.5},
		Memory:     prober.MemoryMetrics{TotalMiB: 16384, UsedMiB: 8192, UsagePercent: 50.0},
		Disk:       prober.DiskResult{Disks: []prober.DiskMetrics{{Mount: "/", SizeGB: 100, UsedGB: 50, UsagePercent: 50.0}}},
		Uptime:     prober.UptimeMetrics{Seconds: 943320},
		CollectedAt: time.Unix(0, 1717900000000000000),
	}
	if err := w.Write(context.Background(), "h-1", m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body := string(rec.body)
	if !strings.Contains(body, "physicalhost_cpu,host_id=h-1 cores=8i") {
		t.Errorf("missing CPU line; body=%s", body)
	}
	if !strings.Contains(body, "physicalhost_memory,host_id=h-1 total_mib=16384i") {
		t.Errorf("missing memory line; body=%s", body)
	}
	if !strings.Contains(body, "physicalhost_uptime,host_id=h-1 seconds=943320i") {
		t.Errorf("missing uptime line; body=%s", body)
	}
	if !strings.Contains(body, "physicalhost_disk,host_id=h-1,mount=/ size_gb=100i") {
		t.Errorf("missing disk line; body=%s", body)
	}
	if !strings.Contains(body, " 1717900000000000000") {
		t.Errorf("missing nanosecond timestamp; body=%s", body)
	}
}

// TestInfluxWriter_SkipsEmpty verifies an empty Metrics
// (total-failure probe) produces no line-protocol body.
func TestInfluxWriter_SkipsEmpty(t *testing.T) {
	rec := &recordingClient{}
	w := NewInfluxWriter(InfluxWriterConfig{URL: "http://x"})
	w.SetClient(rec)
	if err := w.Write(context.Background(), "h-1", Metrics{CollectedAt: time.Now()}); err != nil {
		t.Errorf("Write: %v", err)
	}
	if len(rec.body) != 0 {
		t.Errorf("empty Metrics should produce no lines, got %q", rec.body)
	}
}
