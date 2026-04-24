package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/metrics"
)

type Manager struct {
	mu           sync.RWMutex
	channels     map[string]*Channel
	history      []*Alert
	maxHistory   int
	rateLimiter  *RateLimiter
	metrics      *metrics.Collector
}

type Channel struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	URL       string            `json:"url,omitempty"`
	WebhookURL string           `json:"webhook_url,omitempty"`
	SlackToken string          `json:"slack_token,omitempty"`
	Config    map[string]string `json:"config,omitempty"`
}

type Alert struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Severity  string            `json:"severity"`
	Message   string            `json:"message"`
	Channel   string            `json:"channel"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type RateLimiter struct {
	mu       sync.RWMutex
	windowMs int
	max      int
	counts   map[string][]time.Time
}

func NewRateLimiter(windowMs, max int) *RateLimiter {
	return &RateLimiter{
		windowMs: windowMs,
		max:      max,
		counts:   make(map[string][]time.Time),
	}
}

func (r *RateLimiter) Allow(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	window := time.Duration(r.windowMs) * time.Millisecond
	cutoff := now.Add(-window)

	// Clean old entries
	var valid []time.Time
	for _, t := range r.counts[name] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	r.counts[name] = valid

	if len(r.counts[name]) >= r.max {
		return false
	}

	r.counts[name] = append(r.counts[name], now)
	return true
}

func NewManager(m *metrics.Collector) *Manager {
	return &Manager{
		channels:    make(map[string]*Channel),
		history:     make([]*Alert, 0),
		maxHistory:  1000,
		rateLimiter: NewRateLimiter(60000, 10),
		metrics:     m,
	}
}

func (m *Manager) AddChannel(ch *Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.Name] = ch
	return nil
}

func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chans := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		chans = append(chans, ch)
	}
	return chans
}

func (m *Manager) DeleteChannel(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[name]; ok {
		delete(m.channels, name)
		return true
	}
	return false
}

func (m *Manager) TriggerAlert(name, severity, message string, labels map[string]string) error {
	if !m.rateLimiter.Allow(name) {
		return fmt.Errorf("rate limit exceeded for alert %s", name)
	}

	alert := &Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Name:      name,
		Severity:  severity,
		Message:   message,
		Channel:   name,
		Timestamp: time.Now(),
		Labels:    labels,
	}

	m.mu.Lock()
	m.history = append(m.history, alert)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	// Send to channel
	go m.sendAlert(alert)

	// Record metric
	m.metrics.RecordAlert(name, severity)

	return nil
}

func (m *Manager) sendAlert(alert *Alert) {
	m.mu.RLock()
	ch, ok := m.channels[alert.Name]
	m.mu.RUnlock()
	if !ok {
		return
	}

	switch ch.Type {
	case "webhook":
		m.sendWebhook(ch, alert)
	case "slack":
		m.sendSlack(ch, alert)
	case "log":
		fmt.Printf("[ALERT] %s: %s\n", alert.Severity, alert.Message)
	}
}

func (m *Manager) sendWebhook(ch *Channel, alert *Alert) {
	// Simple HTTP POST implementation
	payload := map[string]interface{}{
		"alert": alert,
	}
	data, _ := json.Marshal(payload)
	fmt.Printf("Webhook would send to %s: %s\n", ch.WebhookURL, string(data))
}

func (m *Manager) sendSlack(ch *Channel, alert *Alert) {
	fmt.Printf("Slack would send: [%s] %s - %s\n", alert.Severity, alert.Name, alert.Message)
}

func (m *Manager) GetHistory() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]*Alert, len(m.history))
	copy(history, m.history)
	return history
}

// HTTP handlers
func (m *Manager) ListChannelsHTTP(w http.ResponseWriter, r *http.Request) {
	chans := m.ListChannels()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chans)
}

func (m *Manager) AddChannelHTTP(w http.ResponseWriter, r *http.Request) {
	var ch Channel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := m.AddChannel(&ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (m *Manager) GetHistoryHTTP(w http.ResponseWriter, r *http.Request) {
	history := m.GetHistory()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}
