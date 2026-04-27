package logs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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
	backend       StorageBackend
	onLogAdded    func(*Entry)
	mu            sync.RWMutex
	entries       []*Entry
	alertRules    []*AlertRule
	retentionDays int
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
		backend:       backend,
		onLogAdded:    onLogAdded,
		entries:       make([]*Entry, 0),
		alertRules:    make([]*AlertRule, 0),
		retentionDays: cfg.RetentionDays,
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

// HTTP handlers
func (m *Manager) QueryLogsHTTP(w http.ResponseWriter, r *http.Request) {
	opts := QueryOptions{
		Level:  r.URL.Query().Get("level"),
		Source: r.URL.Query().Get("source"),
		Search: r.URL.Query().Get("search"),
	}

	entries, err := m.QueryLogs(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": entries, "total": len(entries)})
}

func (m *Manager) CreateLogHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Level   string                 `json:"level"`
		Message string                 `json:"message"`
		Source  string                 `json:"source"`
		Meta    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry, err := m.AddLog(input.Level, input.Message, input.Source, input.Meta)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (m *Manager) GetStatsHTTP(w http.ResponseWriter, r *http.Request) {
	stats, err := m.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rule := m.CreateAlertRule(input.Name, input.Level, input.Pattern, input.Threshold)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
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
}

func NewElasticsearchBackend(cfg LogsConfig) *ElasticsearchBackend {
	return &ElasticsearchBackend{
		url:    cfg.ESURL,
		index:  "devops-logs",
	}
}

func (b *ElasticsearchBackend) Write(entry *Entry) error {
	return fmt.Errorf("elasticsearch backend not implemented")
}

func (b *ElasticsearchBackend) Query(opts QueryOptions) ([]*Entry, error) {
	return nil, fmt.Errorf("elasticsearch backend not implemented")
}

func (b *ElasticsearchBackend) GetStats() (*Stats, error) {
	return nil, fmt.Errorf("elasticsearch backend not implemented")
}

type LokiBackend struct {
	url string
}

func NewLokiBackend(cfg LogsConfig) *LokiBackend {
	return &LokiBackend{url: cfg.LokiURL}
}

func (b *LokiBackend) Write(entry *Entry) error {
	return fmt.Errorf("loki backend not implemented")
}

func (b *LokiBackend) Query(opts QueryOptions) ([]*Entry, error) {
	return nil, fmt.Errorf("loki backend not implemented")
}

func (b *LokiBackend) GetStats() (*Stats, error) {
	return nil, fmt.Errorf("loki backend not implemented")
}
