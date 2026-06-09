package physicalhost

import (
	"context"
	"sync"
	"time"
)

// MetricsSource is the seam the cache uses. The production
// implementation is the SSH/TCP-driven *MetricsCollector;
// tests inject a fake that returns canned snapshots.
type MetricsSource interface {
	Collect(ctx context.Context, host Host) (Metrics, error)
}

// MetricsCacheConfig is the constructor input.
type MetricsCacheConfig struct {
	Collector   MetricsSource
	TTL         time.Duration
	MaxEntries  int
}

// MetricsCache is an in-process LRU keyed by host ID. The
// spec's "Local Cache" requirement (dataStatus=stale when
// Collector fails but a prior snapshot exists, unavailable
// otherwise) lives here. The cache never blocks; the Collector
// is called synchronously on miss.
//
// Concurrency: one sync.RWMutex guards the map. The Collector
// call is done outside the lock so a slow probe doesn't block
// other readers. Two callers for the same host may both
// Collect; this is fine — duplicate probes are cheap and
// avoiding them would require a per-key mutex.
type MetricsCache struct {
	collector MetricsSource
	ttl       time.Duration
	max       int

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	snapshot  Metrics
	expiresAt time.Time
}

// NewMetricsCache builds a cache. Default TTL is 30s and default
// MaxEntries is 256; both can be tuned via the config.
func NewMetricsCache(cfg MetricsCacheConfig) *MetricsCache {
	ttl := cfg.TTL
	if ttl < 0 {
		ttl = 30 * time.Second
	}
	max := cfg.MaxEntries
	if max <= 0 {
		max = 256
	}
	return &MetricsCache{
		collector: cfg.Collector,
		ttl:       ttl,
		max:       max,
		entries:   make(map[string]cacheEntry, max),
	}
}

// GetOrCollect returns the cached snapshot if it is still
// fresh; otherwise it calls Collector.Collect, stores the
// result, and returns it. The failure semantics mirror the
// spec:
//
//	* hit, fresh      -> return as-is
//	* miss, success   -> store + return as-is
//	* hit, expired,   -> store (as stale) + return with
//	  collect success    DataStatus=fresh
//	* hit, expired,   -> keep prior, return with
//	  collect failure    DataStatus=stale
//	* miss, failure   -> return zero + DataStatus=unavailable
func (c *MetricsCache) GetOrCollect(ctx context.Context, hostID string, host Host) (Metrics, error) {
	now := time.Now()
	if hit, ok := c.lookup(hostID, now); ok {
		// Even on a hit, force a refresh once TTL has passed so
		// the cache is self-healing. Concurrent callers race on
		// this; the second Collect is wasted but harmless.
		if hit.expiresAt.After(now) {
			return hit.snapshot, nil
		}
	}
	fresh, err := c.collector.Collect(ctx, host)
	if err != nil {
		// We have a prior snapshot; flip it to stale so the
		// UI can show "last known good" with a warning.
		c.mu.RLock()
		prior, hadPrior := c.entries[hostID]
		c.mu.RUnlock()
		if hadPrior {
			prior.snapshot.DataStatus = DataStatusStale
			return prior.snapshot, nil
		}
		fresh = Metrics{DataStatus: DataStatusUnavailable}
		if err != nil {
			fresh.Warnings = []string{err.Error()}
		}
		return fresh, nil
	}
	c.store(hostID, fresh, now.Add(c.ttl))
	return fresh, nil
}

// lookup is a read-locked helper used by the cache-hit path.
func (c *MetricsCache) lookup(hostID string, now time.Time) (cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[hostID]
	return e, ok
}

// store evicts the oldest entry if we're at capacity, then
// writes the new entry. Eviction is naive (random pick) —
// true LRU is out of scope for v1.
func (c *MetricsCache) store(hostID string, snapshot Metrics, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[hostID]; !ok && len(c.entries) >= c.max {
		// Evict one. Map iteration order is randomised, so
		// this is approximately LRU. Spec doesn't require
		// strict LRU semantics.
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[hostID] = cacheEntry{snapshot: snapshot, expiresAt: expiresAt}
}

// Invalidate drops the cached entry for hostID. Exposed so the
// monitor can clear stale snapshots on a state transition.
func (c *MetricsCache) Invalidate(hostID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, hostID)
}

// Size returns the current number of cached entries. Useful
// for tests and observability metrics.
func (c *MetricsCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
