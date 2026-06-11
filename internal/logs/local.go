package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LocalConfig tunes the in-memory + on-disk Local backend. Dir is
// the directory scanned for *.log files. Readonly disables the
// /_test/echo push endpoint; the service layer checks Readonly
// before calling Append so production deployments can leave echo
// disabled while keeping the Local backend in dev.
type LocalConfig struct {
	Dir      string
	Readonly bool
}

// Local is the file-based log backend. It scans a directory of
// *.log files at query time and matches in-memory. It is the
// in-test backend: the spec calls it out as the fallback when no
// real storage is configured, and tests use it instead of the
// network-bound ES and Loki backends.
//
// The struct is concurrency-safe: a sync.RWMutex guards the
// in-memory echo entries. File scans happen on every query — fine
// for the small fixtures we expect, and it keeps the backend
// stateless so it can be shared across handlers.
type Local struct {
	cfg LocalConfig

	mu    sync.RWMutex
	echo  []LogEntry
	cache []LogEntry // last scan result
	cacheAt time.Time
}

// NewLocal constructs a Local backend rooted at cfg.Dir. An empty
// Dir is allowed (echo-only mode) but a backend with no source will
// always return empty results — callers should set LOG_STORAGE_BACKEND
// in main.go to choose Local for the production /api/v1/logs.
func NewLocal(cfg LocalConfig) *Local {
	return &Local{cfg: cfg}
}

// Capabilities returns the fixed Local capability row. Per the spec
// matrix: no aggregation, 7d max time range, MaxQueryLength 1024.
func (l *Local) Capabilities() Capabilities {
	return Capabilities{
		SupportsAggregation: false,
		MaxTimeRange:        7 * 24 * time.Hour,
		MaxQueryLength:      1024,
		BackendName:         "local",
	}
}

// Append adds an in-memory log entry. The dev-only /_test/echo
// handler uses this; the service layer guards it on cfg.Readonly
// and on dev mode. It also satisfies the LogSink interface used
// by the K8s log-stream path: the K8s streamer fans every line
// through this method via *Service.CreateLogEntry so a developer
// running the Local backend can grep past lines without a
// Loki / ES connection.
func (l *Local) Append(e LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	l.echo = append(l.echo, e)
	l.cache = nil
	return nil
}

// Query scans the configured directory (and any appended echo
// entries), filters by the universal Query, and returns the
// matching entries sorted by time. The meta block always reports
// the backend name; Degraded is set only when the request asked
// for something the Local backend does not support and the
// service layer has narrowed.
func (l *Local) Query(ctx context.Context, q Query) (Result, error) {
	entries, err := l.scan(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("local scan: %w", err)
	}
	filtered := filterEntries(entries, q)

	// Sort: "time:asc" or "time:desc" (default desc).
	sort.SliceStable(filtered, func(i, j int) bool {
		if q.Sort == "time:asc" {
			return filtered[i].Timestamp.Before(filtered[j].Timestamp)
		}
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply limit.
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	total := int64(len(filtered))
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	// Guarantee a non-nil slice so the wire shape is always []
	// rather than null.
	if filtered == nil {
		filtered = []LogEntry{}
	}
	return Result{
		Entries: filtered,
		Total:   total,
		Meta: Meta{
			Backend: "local",
			Limits: map[string]any{
				"max_page_size":  1000,
				"max_time_range": (7 * 24 * time.Hour).String(),
			},
		},
	}, nil
}

// Streams returns the unique source names seen in the scanned
// files plus any echo entries.
func (l *Local) Streams(ctx context.Context) ([]Stream, error) {
	entries, err := l.scan(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]Stream{}
	for _, e := range entries {
		if e.Source == "" {
			continue
		}
		if _, ok := seen[e.Source]; !ok {
			seen[e.Source] = Stream{Name: e.Source}
		}
	}
	out := make([]Stream, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// scan returns the union of disk-scanned entries and any echo
// entries. Results are cached briefly so a burst of queries
// against the same data set doesn't re-read the disk.
func (l *Local) scan(ctx context.Context) ([]LogEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.RLock()
	if l.cache != nil && time.Since(l.cacheAt) < 5*time.Second {
		out := append([]LogEntry(nil), l.cache...)
		l.mu.RUnlock()
		return out, nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	// Re-check after acquiring write lock.
	if l.cache != nil && time.Since(l.cacheAt) < 5*time.Second {
		return append([]LogEntry(nil), l.cache...), nil
	}
	var out []LogEntry
	if l.cfg.Dir != "" {
		disk, err := scanDir(l.cfg.Dir)
		if err != nil {
			return nil, err
		}
		out = append(out, disk...)
	}
	out = append(out, l.echo...)
	l.cache = out
	l.cacheAt = time.Now()
	return append([]LogEntry(nil), out...), nil
}

// scanDir reads every *.log file in dir and parses each line. We
// try JSON first; if that fails, fall back to the plain-text shape
// "2026-06-05T11:59:30Z [level] source: message host=foo".
func scanDir(dir string) ([]LogEntry, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil {
		return nil, err
	}
	var out []LogEntry
	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if e, ok := parseLine(line); ok {
				out = append(out, e)
			}
		}
		f.Close()
	}
	return out, nil
}

// parseLine tries JSON first, then a plain-text format. The
// plain-text fallback is intentionally minimal — the Local
// backend is for dev/test, not for production ingestion.
func parseLine(line string) (LogEntry, bool) {
	if strings.HasPrefix(line, "{") {
		var raw struct {
			ID        string            `json:"id"`
			Timestamp string            `json:"timestamp"`
			Level     string            `json:"level"`
			Source    string            `json:"source"`
			Message   string            `json:"message"`
			Host      string            `json:"host"`
			Labels    map[string]string `json:"labels"`
			Fields    map[string]any    `json:"fields"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			ts, _ := time.Parse(time.RFC3339, raw.Timestamp)
			return LogEntry{
				ID:        raw.ID,
				Timestamp: ts,
				Level:     raw.Level,
				Source:    raw.Source,
				Message:   raw.Message,
				Host:      raw.Host,
				Labels:    raw.Labels,
				Fields:    raw.Fields,
			}, true
		}
	}
	return parsePlainLine(line)
}

// parsePlainLine matches "RFC3339 [level] source: message host=foo".
// Returns the zero LogEntry and false on any parse failure.
func parsePlainLine(line string) (LogEntry, bool) {
	sp := strings.SplitN(line, " ", 3)
	if len(sp) < 3 {
		return LogEntry{}, false
	}
	ts, err := time.Parse(time.RFC3339, sp[0])
	if err != nil {
		return LogEntry{}, false
	}
	rest := sp[2]
	level := ""
	if strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]")
		if end < 0 {
			return LogEntry{}, false
		}
		level = rest[1:end]
		rest = strings.TrimSpace(rest[end+1:])
	}
	src := ""
	if i := strings.Index(rest, ":"); i >= 0 {
		src = rest[:i]
		rest = strings.TrimSpace(rest[i+1:])
	}
	host := ""
	msg := rest
	if i := strings.Index(rest, " host="); i >= 0 {
		msg = strings.TrimSpace(rest[:i])
		host = rest[i+len(" host="):]
	}
	return LogEntry{
		Timestamp: ts,
		Level:     level,
		Source:    src,
		Message:   msg,
		Host:      host,
	}, true
}

// filterEntries applies the universal subset of the Query: time
// range, substring (case-insensitive), and the level / source /
// host filters. Unknown filter keys are ignored (the service layer
// is the gatekeeper for advanced features).
func filterEntries(in []LogEntry, q Query) []LogEntry {
	needle := strings.ToLower(q.Text)
	var out []LogEntry
	for _, e := range in {
		if !q.From.IsZero() && e.Timestamp.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && e.Timestamp.After(q.To) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(e.Message), needle) {
			continue
		}
		if !matchFilters(e, q.Filters) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// matchFilters applies each Filter. Supported ops are eq, neq,
// prefix, suffix, contains, regex. An unknown op is a non-match
// (the service layer should reject those before they get here).
func matchFilters(e LogEntry, filters []Filter) bool {
	for _, f := range filters {
		actual := lookupField(e, f.Key)
		if !applyOp(actual, f.Op, f.Value) {
			return false
		}
	}
	return true
}

// lookupField returns the value of key in the LogEntry. Recognised
// keys: level, source, host, message. Anything else returns "".
func lookupField(e LogEntry, key string) string {
	switch key {
	case "level":
		return e.Level
	case "source":
		return e.Source
	case "host":
		return e.Host
	case "message":
		return e.Message
	}
	return ""
}

func applyOp(actual, op, want string) bool {
	switch op {
	case "eq", "":
		return actual == want
	case "neq":
		return actual != want
	case "prefix":
		return strings.HasPrefix(actual, want)
	case "suffix":
		return strings.HasSuffix(actual, want)
	case "contains":
		return strings.Contains(actual, want)
	case "regex":
		// Regex on Local is rejected at the service layer; we
		// just return false here so a non-match is a no-op.
		return false
	}
	return false
}
