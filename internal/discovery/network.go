package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
)

type Manager struct {
	mu       sync.RWMutex
	status   *ScanStatus
	results  []*DiscoveredDevice
}

type ScanStatus struct {
	InProgress bool      `json:"in_progress"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Targets    int       `json:"targets"`
	Scanned    int       `json:"scanned"`
}

type DiscoveredDevice struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	Name            string            `json:"name"`
	IP              string            `json:"ip"`
	Port            int               `json:"port"`
	Labels          map[string]string `json:"labels"`
	DiscoveryMethod string            `json:"discovery_method"`
}

func NewManager() *Manager {
	return &Manager{
		status:  &ScanStatus{},
		results: make([]*DiscoveredDevice, 0),
	}
}

func (m *Manager) GetStatus() *ScanStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) Scan(targets []string) error {
	m.mu.Lock()
	m.status = &ScanStatus{
		InProgress: true,
		StartedAt:  time.Now(),
		Targets:    len(targets),
		Scanned:    0,
	}
	m.mu.Unlock()

	go m.performScan(targets)

	return nil
}

func (m *Manager) performScan(targets []string) {
	for _, target := range targets {
		// Simulate SNMP/SSH discovery
		time.Sleep(100 * time.Millisecond)

		device := &DiscoveredDevice{
			ID:              fmt.Sprintf("disc-%d", time.Now().UnixNano()),
			Type:            "network_device",
			Name:            fmt.Sprintf("discovered-%s", target),
			IP:              target,
			Port:            22,
			Labels:          map[string]string{},
			DiscoveryMethod:  "ssh",
		}

		m.mu.Lock()
		m.results = append(m.results, device)
		m.status.Scanned++
		m.mu.Unlock()
	}

	m.mu.Lock()
	m.status.InProgress = false
	m.status.FinishedAt = time.Now()
	m.mu.Unlock()
}

func (m *Manager) GetResults() []*DiscoveredDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]*DiscoveredDevice, len(m.results))
	copy(results, m.results)
	return results
}

func (m *Manager) RegisterDevice(device *DiscoveredDevice) error {
	// Registration logic would add to device manager
	return nil
}

// HTTP handlers
func (m *Manager) GetStatusHTTP(w http.ResponseWriter, r *http.Request) {
	status := m.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (m *Manager) GetResultsHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	results := m.GetResults()
	// Apply pagination in-memory
	total := len(results)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedResults := results[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedResults, total, limit, offset))
}

func (m *Manager) ScanHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Targets []string `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	if err := m.Scan(input.Targets); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "scan started"})
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
