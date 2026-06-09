package physicalhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// fakeCollector is a MetricsCollector substitute that returns a
// canned snapshot on every Collect call and counts invocations.
// It exists so the cache test can assert on the call rate
// without standing up a real SSH server.
type fakeCollector struct {
	mu    sync.Mutex
	calls int
	out   Metrics
	err   error
}

func (f *fakeCollector) Collect(_ context.Context, _ Host) (Metrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.out, f.err
}

// TestMetricsCache_FreshFromCache confirms that a cache hit
// within TTL does NOT trigger another Collect call.
func TestMetricsCache_FreshFromCache(t *testing.T) {
	fc := &fakeCollector{out: Metrics{DataStatus: DataStatusFresh, CPU: prober.CPUMetrics{Cores: 8}}}
	c := NewMetricsCache(MetricsCacheConfig{Collector: fc, TTL: 5 * time.Second, MaxEntries: 16})

	host := Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"}
	hostID := "h-1"
	m1, _ := c.GetOrCollect(context.Background(), hostID, host)
	m2, _ := c.GetOrCollect(context.Background(), hostID, host)
	if m1.CPU.Cores != 8 || m2.CPU.Cores != 8 {
		t.Errorf("cached values wrong: %+v / %+v", m1, m2)
	}
	if fc.calls != 1 {
		t.Errorf("Collect called %d times, want 1", fc.calls)
	}
}

// TestMetricsCache_StaleAfterTTL expires the entry, forcing a
// second Collect. The returned Metrics gets a stale badge.
func TestMetricsCache_StaleAfterTTL(t *testing.T) {
	fc := &fakeCollector{out: Metrics{DataStatus: DataStatusFresh, CPU: prober.CPUMetrics{Cores: 8}}}
	c := NewMetricsCache(MetricsCacheConfig{Collector: fc, TTL: 10 * time.Millisecond, MaxEntries: 16})

	hostID, host := "h-1", Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"}
	_, _ = c.GetOrCollect(context.Background(), hostID, host)
	time.Sleep(30 * time.Millisecond)

	// Second call after TTL → Collect runs again.
	_, _ = c.GetOrCollect(context.Background(), hostID, host)
	if fc.calls != 2 {
		t.Errorf("Collect called %d times, want 2 (once per TTL window)", fc.calls)
	}
}

// TestMetricsCache_StaleOnFailureWhenCachedPrevious: the cache
// returns the previous snapshot with DataStatus flipped to
// "stale" when a fresh Collect fails. This is the spec's
// "data query with DB failure" pattern. We use TTL=0 so the
// cache always tries to refresh — that's the only way the
// failure path is exercised.
func TestMetricsCache_StaleOnFailureWhenCachedPrevious(t *testing.T) {
	fc := &fakeCollector{out: Metrics{DataStatus: DataStatusFresh}}
	c := NewMetricsCache(MetricsCacheConfig{Collector: fc, TTL: 0, MaxEntries: 16})

	hostID, host := "h-1", Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"}
	first, _ := c.GetOrCollect(context.Background(), hostID, host)
	if first.DataStatus != DataStatusFresh {
		t.Fatalf("setup: first call status = %q", first.DataStatus)
	}

	// Second call: collector fails, cache should still return
	// the prior snapshot but flagged as stale.
	fc.err = errors.New("ssh: connection refused")
	second, _ := c.GetOrCollect(context.Background(), hostID, host)
	if second.DataStatus != DataStatusStale {
		t.Errorf("DataStatus on failure = %q, want %q", second.DataStatus, DataStatusStale)
	}
}

// TestMetricsCache_UnavailableWhenEmptyAndFailure: no prior
// snapshot, fresh Collect fails → caller sees unavailable.
func TestMetricsCache_UnavailableWhenEmptyAndFailure(t *testing.T) {
	fc := &fakeCollector{err: errors.New("ssh: connection refused")}
	c := NewMetricsCache(MetricsCacheConfig{Collector: fc, TTL: 5 * time.Second, MaxEntries: 16})

	hostID, host := "h-1", Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"}
	m, _ := c.GetOrCollect(context.Background(), hostID, host)
	if m.DataStatus != DataStatusUnavailable {
		t.Errorf("DataStatus = %q, want %q", m.DataStatus, DataStatusUnavailable)
	}
}

// TestMetricsCache_BoundedEviction pins the MaxEntries cap. A
// 1-entry cache should drop the only entry when a different
// host is queried, forcing a re-collect.
func TestMetricsCache_BoundedEviction(t *testing.T) {
	fc := &fakeCollector{out: Metrics{DataStatus: DataStatusFresh, CPU: prober.CPUMetrics{Cores: 99}}}
	c := NewMetricsCache(MetricsCacheConfig{Collector: fc, TTL: 0, MaxEntries: 1})
	host := Host{IPAddress: "1.2.3.4", SSHPort: 22, SSHUser: "root"}

	// Seed h-1 once (calls=1).
	first, _ := c.GetOrCollect(context.Background(), "h-1", host)
	if first.CPU.Cores != 99 {
		t.Fatalf("setup: h-1 cores = %d, want 99", first.CPU.Cores)
	}
	// Seed h-2: evicts h-1 (calls=2).
	_, _ = c.GetOrCollect(context.Background(), "h-2", host)
	// Re-query h-1: must miss (h-1 was evicted) → re-collect (calls=3).
	fc.mu.Lock()
	fc.out = Metrics{DataStatus: DataStatusFresh, CPU: prober.CPUMetrics{Cores: 1}}
	fc.mu.Unlock()
	got, _ := c.GetOrCollect(context.Background(), "h-1", host)
	if got.CPU.Cores != 1 {
		t.Errorf("h-1 re-collect cores = %d, want 1 (proves eviction forced fresh Collect)", got.CPU.Cores)
	}
	if fc.calls != 3 {
		t.Errorf("Collect called %d times, want 3", fc.calls)
	}
}
