package physicalhost

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeProber is a minimal Prober substitute. It avoids the full
// Fake in prober_test.go (which lives in a subpackage) so the
// metrics test stays self-contained. Scripted() returns the
// canned stdout for a given command; an unprogrammed command
// returns the err.
type fakeProber struct {
	cmds map[string][]byte
	err  error
}

func (f *fakeProber) Ping(_ context.Context, _ Host) (PingResult, error) {
	return PingResult{Reachable: true}, nil
}
func (f *fakeProber) SSHExec(_ context.Context, _ Host, cmd string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if out, ok := f.cmds[cmd]; ok {
		return out, nil
	}
	return nil, errors.New("unknown cmd: " + cmd)
}

const sampleFree = `              total        used        free      shared  buff/cache   available
Mem:          16384        8192        4096         256        4096        7584
Swap:          2048         512        1536
`

const sampleDF = `Filesystem     1G-blocks  Used Available Use% Mounted on
/dev/sda1             100G    50G        50G  50% /
`

const sampleTop = `%Cpu(s): 12.5 us,  3.2 sy,  0.0 ni, 84.3 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
`

// TestMetricsCollector_HappyPath verifies the metrics service
// drives the prober through all four commands and produces a
// fully populated Metrics struct.
func TestMetricsCollector_HappyPath(t *testing.T) {
	p := &fakeProber{cmds: map[string][]byte{
		"nproc":                 []byte("8\n"),
		"top -bn1 | head -5":    []byte(sampleTop),
		"free -m":               []byte(sampleFree),
		"df -BG":                []byte(sampleDF),
		"uptime -p":             []byte("up 10 days, 3 hours, 42 minutes"),
		"cat /proc/uptime":      []byte("943320.00 1234567.89"),
	}}
	mc := NewMetricsCollector(MetricsCollectorConfig{Prober: p, Timeout: 2 * time.Second})

	m, err := mc.Collect(context.Background(), Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if m.DataStatus != DataStatusFresh {
		t.Errorf("DataStatus = %q, want %q", m.DataStatus, DataStatusFresh)
	}
	if m.CPU.Cores != 8 {
		t.Errorf("CPU.Cores = %d, want 8", m.CPU.Cores)
	}
	if m.CPU.UsagePercent < 15.5 || m.CPU.UsagePercent > 16.0 {
		t.Errorf("CPU.UsagePercent = %f, want ~15.7", m.CPU.UsagePercent)
	}
	if m.Memory.TotalMiB != 16384 || m.Memory.UsedMiB != 8192 {
		t.Errorf("Memory = %+v, want total=16384 used=8192", m.Memory)
	}
	if len(m.Disk.Disks) != 1 || m.Disk.Disks[0].Mount != "/" {
		t.Errorf("Disk = %+v, want one / mount", m.Disk)
	}
	if m.Uptime.Seconds < 943300 {
		t.Errorf("Uptime.Seconds = %d, want ~943320", m.Uptime.Seconds)
	}
	if m.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}
}

// TestMetricsCollector_PartialFailure still returns Metrics —
// the spec says the UI degrades gracefully. Disk times out but
// CPU/mem/uptime come through. DataStatus should mark "stale".
func TestMetricsCollector_PartialFailure(t *testing.T) {
	p := &fakeProber{cmds: map[string][]byte{
		"nproc":              []byte("4\n"),
		"top -bn1 | head -5": []byte(sampleTop),
		"free -m":            []byte(sampleFree),
		// "df -BG" deliberately not scripted → fake returns "unknown cmd"
		"uptime -p":        []byte("up 1 hour"),
		"cat /proc/uptime": []byte("3600.00 0.00"),
	}}
	mc := NewMetricsCollector(MetricsCollectorConfig{Prober: p, Timeout: 1 * time.Second})

	m, err := mc.Collect(context.Background(), Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"})
	if err != nil {
		t.Fatalf("Collect should swallow partial errors, got %v", err)
	}
	if m.DataStatus != DataStatusStale {
		t.Errorf("DataStatus = %q, want %q", m.DataStatus, DataStatusStale)
	}
	if m.CPU.Cores != 4 {
		t.Errorf("CPU.Cores = %d, want 4 (CPU still parsed)", m.CPU.Cores)
	}
	if len(m.Disk.Disks) != 0 {
		t.Errorf("Disk should be empty, got %+v", m.Disk.Disks)
	}
	if m.Memory.TotalMiB != 16384 {
		t.Errorf("Memory should still parse, got total=%d", m.Memory.TotalMiB)
	}
	if len(m.Warnings) == 0 {
		t.Error("expected at least one warning for the failed disk command")
	}
}

// TestMetricsCollector_TotalFailure returns Metrics with
// DataStatus=unavailable; the cache layer uses this to
// distinguish "no data" from "stale data".
func TestMetricsCollector_TotalFailure(t *testing.T) {
	p := &fakeProber{err: errors.New("ssh: connection refused")}
	mc := NewMetricsCollector(MetricsCollectorConfig{Prober: p, Timeout: 100 * time.Millisecond})

	m, err := mc.Collect(context.Background(), Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"})
	if err != nil {
		t.Fatalf("Collect should never bubble errors up, got %v", err)
	}
	if m.DataStatus != DataStatusUnavailable {
		t.Errorf("DataStatus = %q, want %q", m.DataStatus, DataStatusUnavailable)
	}
	if m.CPU.Cores != 0 || m.Memory.TotalMiB != 0 {
		t.Errorf("expected zero metrics on total failure, got %+v", m)
	}
	if len(m.Warnings) == 0 {
		t.Error("expected warnings describing the failure")
	}
}
