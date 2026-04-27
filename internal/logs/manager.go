package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
	"github.com/google/uuid"
)

type Entry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time               `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Resource  string                 `json:"resource,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
}

type Manager struct {
	backend          StorageBackend
	onLogAdded       func(*Entry)
	mu               sync.RWMutex
	entries          []*Entry
	alertRules       []*AlertRule
	savedFilters     []*SavedFilter
	retentionPolicy  *RetentionPolicy
	retentionDays    int
}

type StorageBackend interface {
	Write(entry *Entry) error
	Query(opts QueryOptions) ([]*Entry, error)
	GetStats() (*Stats, error)
}

type QueryOptions struct {
	Level    string
	Source   string
	Search   string
	Resource string
	Tags     []string
	Order    string
	Limit    int
	Offset   int
}

type AlertRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Level     string    `json:"level"`
	Pattern   string    `json:"pattern"`
	Threshold int       `json:"threshold"`
	WindowMs  int       `json:"window_ms"`
	Enabled   bool      `json:"enabled"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SavedFilter struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Level     string                 `json:"level,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Search    string                 `json:"search,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type RetentionPolicy struct {
	Days           int       `json:"days"`
	MaxLogs        int       `json:"max_logs,omitempty"`
	ApplyEnabled   bool      `json:"apply_enabled"`
	LastAppliedAt  time.Time `json:"last_applied_at,omitempty"`
	LastAppliedBy  string    `json:"last_applied_by,omitempty"`
}

type Stats struct {
	Total     int            `json:"total"`
	ByLevel   map[string]int `json:"by_level"`
	BySource  map[string]int `json:"by_source"`
	LastHour  int            `json:"last_hour"`
	Last24h   int            `json:"last_24h"`
	ErrorRate float64       `json:"error_rate"`
}

func NewManager(cfg LogsConfig, onLogAdded func(*Entry)) *Manager {
	var backend StorageBackend
	switch cfg.Backend {
	case "elasticsearch":
		backend = NewElasticsearchBackend(cfg)
	case "loki":
		backend = NewLokiBackend(cfg)
	default:
		backend = NewLocalBackend(cfg)
	}

	return &Manager{
		backend:         backend,
		onLogAdded:      onLogAdded,
		entries:         make([]*Entry, 0),
		alertRules:      make([]*AlertRule, 0),
		savedFilters:    make([]*SavedFilter, 0),
		retentionPolicy: &RetentionPolicy{Days: cfg.RetentionDays, ApplyEnabled: true},
		retentionDays:   cfg.RetentionDays,
	}
}

func (m *Manager) AddLog(level, message, source string, meta map[string]interface{}) (*Entry, error) {
	entry := &Entry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
		Metadata:  meta,
		Tags:      []string{},
	}

	if err := m.backend.Write(entry); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.entries = append(m.entries, entry)
	m.mu.Unlock()

	if m.onLogAdded != nil {
		m.onLogAdded(entry)
	}

	return entry, nil
}

func (m *Manager) QueryLogs(opts QueryOptions) ([]*Entry, error) {
	return m.backend.Query(opts)
}

func (m *Manager) GetStats() (*Stats, error) {
	return m.backend.GetStats()
}

func (m *Manager) CreateAlertRule(name, level, pattern string, threshold int) *AlertRule {
	rule := &AlertRule{
		ID:        uuid.New().String(),
		Name:      name,
		Level:     level,
		Pattern:   pattern,
		Threshold: threshold,
		WindowMs:  60000,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	m.mu.Lock()
	m.alertRules = append(m.alertRules, rule)
	m.mu.Unlock()
	return rule
}

func (m *Manager) ListAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertRules
}

func (m *Manager) DeleteAlertRule(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.alertRules {
		if r.ID == id {
			m.alertRules = append(m.alertRules[:i], m.alertRules[i+1:]...)
			return true
		}
	}
	return false
}

// Retention policy management
func (m *Manager) GetRetentionPolicy() *RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.retentionPolicy
}

func (m *Manager) UpdateRetentionPolicy(days int, maxLogs int, applyEnabled bool) *RetentionPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retentionPolicy.Days = days
	m.retentionPolicy.MaxLogs = maxLogs
	m.retentionPolicy.ApplyEnabled = applyEnabled
	return m.retentionPolicy
}

func (m *Manager) ApplyRetentionPolicy() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.retentionPolicy.Days)
	originalCount := len(m.entries)

	var kept []*Entry
	for _, e := range m.entries {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		}
	}
	m.entries = kept

	m.retentionPolicy.LastAppliedAt = time.Now()
	return originalCount - len(m.entries), nil
}

// Saved filter management
func (m *Manager) CreateSavedFilter(name, level, source, search, resource string, tags []string) *SavedFilter {
	filter := &SavedFilter{
		ID:        uuid.New().String(),
		Name:      name,
		Level:     level,
		Source:    source,
		Search:    search,
		Resource:  resource,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	m.mu.Lock()
	m.savedFilters = append(m.savedFilters, filter)
	m.mu.Unlock()
	return filter
}

func (m *Manager) ListSavedFilters() []*SavedFilter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.savedFilters
}

func (m *Manager) DeleteSavedFilter(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, f := range m.savedFilters {
		if f.ID == id {
			m.savedFilters = append(m.savedFilters[:i], m.savedFilters[i+1:]...)
			return true
		}
	}
	return false
}

// Sample log generation
func (m *Manager) GenerateSampleLogs(count int) []*Entry {
	levels := []string{"debug", "info", "info", "info", "warning", "error"}
	sources := []string{"api", "web", "worker", "database", "auth", "scheduler"}
	messages := []string{
		"Request processed successfully",
		"Connection established",
		"User authentication completed",
		"Cache miss for key",
		"Rate limit threshold reached",
		"Database query slow",
		"HTTP request timeout",
		"Configuration reloaded",
		"Health check passed",
		"Background job completed",
	}

	entries := make([]*Entry, 0, count)
	for i := 0; i < count; i++ {
		entry := &Entry{
			ID:        uuid.New().String(),
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
			Level:     levels[i%len(levels)],
			Message:   messages[i%len(messages)],
			Source:    sources[i%len(sources)],
			Metadata:  map[string]interface{}{"generated": true, "index": i},
			Tags:      []string{},
		}
		if err := m.backend.Write(entry); err == nil {
			entries = append(entries, entry)
		}
	}

	m.mu.Lock()
	m.entries = append(m.entries, entries...)
	m.mu.Unlock()

	return entries
}

func (m *Manager) CheckAlerts(entry *Entry) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.alertRules {
		if !rule.Enabled {
			continue
		}
		if rule.Level != entry.Level {
			continue
		}
		if rule.Source != "" && rule.Source != entry.Source {
			continue
		}
		if rule.Pattern != "" {
			if !containsPattern(entry.Message, rule.Pattern) {
				continue
			}
		}
		// Alert triggered
	}
}

func containsPattern(message, pattern string) bool {
	return len(pattern) > 0 && len(message) > 0 && strings.Contains(message, pattern)
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// HTTP handlers
func (m *Manager) QueryLogsHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	opts := QueryOptions{
		Level:  r.URL.Query().Get("level"),
		Source: r.URL.Query().Get("source"),
		Search: r.URL.Query().Get("search"),
		Limit:  limit,
		Offset: offset,
	}

	entries, err := m.QueryLogs(opts)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	// For logs, we can't easily get total count without scanning all entries
	// So we use the returned count as a proxy
	total := len(entries) + offset
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(entries, total, limit, offset))
}

func (m *Manager) CreateLogHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Level   string                 `json:"level"`
		Message string                 `json:"message"`
		Source  string                 `json:"source"`
		Meta    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	entry, err := m.AddLog(input.Level, input.Message, input.Source, input.Meta)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (m *Manager) GetStatsHTTP(w http.ResponseWriter, r *http.Request) {
	stats, err := m.GetStats()
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (m *Manager) ListAlertRulesHTTP(w http.ResponseWriter, r *http.Request) {
	rules := m.ListAlertRules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func (m *Manager) CreateAlertRuleHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string `json:"name"`
		Level     string `json:"level"`
		Pattern   string `json:"pattern"`
		Threshold int    `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	rule := m.CreateAlertRule(input.Name, input.Level, input.Pattern, input.Threshold)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// Retention policy HTTP handlers
func (m *Manager) GetRetentionPolicyHTTP(w http.ResponseWriter, r *http.Request) {
	policy := m.GetRetentionPolicy()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

func (m *Manager) UpdateRetentionPolicyHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Days         int  `json:"days"`
		MaxLogs      int  `json:"max_logs"`
		ApplyEnabled bool `json:"apply_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	if input.Days < 1 {
		input.Days = 30 // default
	}

	policy := m.UpdateRetentionPolicy(input.Days, input.MaxLogs, input.ApplyEnabled)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

func (m *Manager) ApplyRetentionPolicyHTTP(w http.ResponseWriter, r *http.Request) {
	deleted, err := m.ApplyRetentionPolicy()
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"status":  "completed",
	})
}

// Saved filter HTTP handlers
func (m *Manager) ListSavedFiltersHTTP(w http.ResponseWriter, r *http.Request) {
	filters := m.ListSavedFilters()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

func (m *Manager) CreateSavedFilterHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string   `json:"name"`
		Level    string   `json:"level"`
		Source   string   `json:"source"`
		Search   string   `json:"search"`
		Resource string   `json:"resource"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	filter := m.CreateSavedFilter(input.Name, input.Level, input.Source, input.Search, input.Resource, input.Tags)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(filter)
}

// Generate sample logs HTTP handler
func (m *Manager) GenerateSampleLogsHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.Count = 10 // default
	}
	if input.Count < 1 || input.Count > 1000 {
		input.Count = 10
	}

	entries := m.GenerateSampleLogs(input.Count)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"generated": len(entries),
		"entries":   entries,
	})
}

type LogsConfig struct {
	Backend       string
	RetentionDays int
	Path          string
	ESURL         string
	LokiURL       string
}

type LocalBackend struct {
	path  string
	mu    sync.RWMutex
	data  struct {
		Logs   []*Entry     `json:"logs"`
		Alerts []*AlertRule `json:"alerts"`
	}
}

func NewLocalBackend(cfg LogsConfig) *LocalBackend {
	return &LocalBackend{path: cfg.Path}
}

func (b *LocalBackend) Write(entry *Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data.Logs = append(b.data.Logs, entry)
	return nil
}

func (b *LocalBackend) Query(opts QueryOptions) ([]*Entry, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	results := []*Entry{}
	for _, e := range b.data.Logs {
		if opts.Level != "" && e.Level != opts.Level {
			continue
		}
		if opts.Source != "" && e.Source != opts.Source {
			continue
		}
		if opts.Search != "" && !containsPattern(e.Message, opts.Search) {
			continue
		}
		results = append(results, e)
	}
	return results, nil
}

func (b *LocalBackend) GetStats() (*Stats, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := &Stats{
		Total:     len(b.data.Logs),
		ByLevel:   make(map[string]int),
		BySource:  make(map[string]int),
		LastHour:  0,
		Last24h:   0,
		ErrorRate: 0,
	}

	now := time.Now()
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)
	errorCount := 0

	for _, e := range b.data.Logs {
		stats.ByLevel[e.Level]++
		stats.BySource[e.Source]++
		if e.Timestamp.After(hourAgo) {
			stats.LastHour++
		}
		if e.Timestamp.After(dayAgo) {
			stats.Last24h++
		}
		if e.Level == "error" {
			errorCount++
		}
	}

	if stats.Total > 0 {
		stats.ErrorRate = float64(errorCount) / float64(stats.Total) * 100
	}

	return stats, nil
}

type ElasticsearchBackend struct {
	url    string
	index  string
	client *http.Client
}

func NewElasticsearchBackend(cfg LogsConfig) *ElasticsearchBackend {
	return &ElasticsearchBackend{
		url:    cfg.ESURL,
		index:  "devops-logs",
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *ElasticsearchBackend) esURL(path string) string {
	return b.url + path
}

func (b *ElasticsearchBackend) Write(entry *Entry) error {
	doc := map[string]interface{}{
		"@timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"level":      entry.Level,
		"message":    entry.Message,
		"source":     entry.Source,
		"resource":   entry.Resource,
		"metadata":   entry.Metadata,
		"tags":       entry.Tags,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	resp, err := http.Post(
		b.esURL("/"+b.index+"/_doc"),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to write to elasticsearch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch error: %s", string(body))
	}

	return nil
}

func (b *ElasticsearchBackend) Query(opts QueryOptions) ([]*Entry, error) {
	query := b.buildQuery(opts)
	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	resp, err := http.Post(
		b.esURL("/"+b.index+"/_search?size=100"),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query elasticsearch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elasticsearch error: %s", string(body))
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	entries := make([]*Entry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		var entry Entry
		if err := json.Unmarshal(hit.Source, &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

func (b *ElasticsearchBackend) buildQuery(opts QueryOptions) map[string]interface{} {
	must := []map[string]interface{}{}

	if opts.Level != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"level": opts.Level},
		})
	}
	if opts.Source != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"source": opts.Source},
		})
	}
	if opts.Search != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{"message": opts.Search},
		})
	}
	if opts.Resource != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"resource": opts.Resource},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]interface{}{"order": "desc"}},
		},
	}

	if len(must) == 0 {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"match_all": map[string]interface{}{},
			},
			"sort": []map[string]interface{}{
				{"@timestamp": map[string]interface{}{"order": "desc"}},
			},
		}
	}

	return query
}

func (b *ElasticsearchBackend) GetStats() (*Stats, error) {
	query := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			"by_level": map[string]interface{}{
				"terms": map[string]interface{}{"field": "level"},
			},
			"by_source": map[string]interface{}{
				"terms": map[string]interface{}{"field": "source"},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	resp, err := http.Post(
		b.esURL("/"+b.index+"/_search"),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats from elasticsearch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elasticsearch error: %s", string(body))
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			ByLevel struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int    `json:"doc_count"`
				} `json:"buckets"`
			} `json:"by_level"`
			BySource struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int    `json:"doc_count"`
				} `json:"buckets"`
			} `json:"by_source"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	stats := &Stats{
		Total:    result.Hits.Total.Value,
		ByLevel:  make(map[string]int),
		BySource: make(map[string]int),
	}

	for _, bucket := range result.Aggregations.ByLevel.Buckets {
		stats.ByLevel[bucket.Key] = bucket.DocCount
	}
	for _, bucket := range result.Aggregations.BySource.Buckets {
		stats.BySource[bucket.Key] = bucket.DocCount
	}

	return stats, nil
}

type LokiBackend struct {
	url    string
	client *http.Client
}

func NewLokiBackend(cfg LogsConfig) *LokiBackend {
	return &LokiBackend{
		url:    cfg.LokiURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *LokiBackend) lokiURL(path string) string {
	return b.url + path
}

func (b *LokiBackend) Write(entry *Entry) error {
	// Loki uses a push-based API with stream labels
	labels := map[string]string{
		"level":  entry.Level,
		"source": entry.Source,
	}
	if entry.Resource != "" {
		labels["resource"] = entry.Resource
	}

	stream := map[string]interface{}{
		"stream": labels,
		"values": [][]string{
			{
				fmt.Sprintf("%d", entry.Timestamp.UnixNano()),
				entry.Message,
			},
		},
	}

	pushRequest := map[string]interface{}{
		"streams": []map[string]interface{}{stream},
	}

	data, err := json.Marshal(pushRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal push request: %w", err)
	}

	resp, err := http.Post(
		b.lokiURL("/loki/api/v1/push"),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to write to loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("loki error: %s", string(body))
	}

	return nil
}

func (b *LokiBackend) Query(opts QueryOptions) ([]*Entry, error) {
	// Build LogQL query
	query := ""
	if opts.Search != "" {
		query = fmt.Sprintf(`{source="%s"} |= "%s"`, opts.Source, opts.Search)
	} else if opts.Level != "" {
		query = fmt.Sprintf(`{source="%s"} |= "%s"`, opts.Source, opts.Level)
	} else {
		query = `{source="` + opts.Source + `"}`
	}

	if query == "" {
		query = "{}"
	}

	url := b.lokiURL("/loki/api/v1/query_range") + "?query=" + query +
		"&limit=" + strconv.Itoa(opts.Limit) +
		"&start=" + fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).UnixNano()) +
		"&end=" + fmt.Sprintf("%d", time.Now().UnixNano())

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki error: %s", string(body))
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode loki response: %w", err)
	}

	entries := make([]*Entry, 0, len(result.Data.Result))
	for _, stream := range result.Data.Result {
		for _, value := range stream.Values {
			if len(value) < 2 {
				continue
			}
			ts, _ := strconv.ParseInt(value[0], 10, 64)
			entry := &Entry{
				ID:        uuid.New().String(),
				Timestamp: time.Unix(0, ts),
				Message:   value[1],
				Level:     stream.Stream["level"],
				Source:    stream.Stream["source"],
				Resource:  stream.Stream["resource"],
				Tags:      []string{},
			}
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (b *LokiBackend) GetStats() (*Stats, error) {
	// Use stats endpoint or aggregate query
	url := b.lokiURL("/loki/api/v1/label/level/values")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get loki stats: %w", err)
	}
	defer resp.Body.Close()

	stats := &Stats{
		ByLevel:  make(map[string]int),
		BySource: make(map[string]int),
	}

	if resp.StatusCode >= 400 {
		// Return empty stats if Loki is not available
		return stats, nil
	}

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return stats, nil
	}

	// Query for total approximate count using a count aggregation
	countQuery := "sum(count_over_time({}[24h]))"
	countURL := b.lokiURL("/loki/api/v1/query") + "?query=" + countQuery

	countResp, err := http.Get(countURL)
	if err != nil {
		return stats, nil
	}
	defer countResp.Body.Close()

	return stats, nil
}
