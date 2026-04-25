// Package discovery tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package discovery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManager_NewManager(t *testing.T) {
	m := NewManager()
	if m.status == nil {
		t.Fatal("expected status to be initialized")
	}
	if m.results == nil {
		t.Fatal("expected results to be initialized")
	}
}

func TestManager_GetStatus(t *testing.T) {
	m := NewManager()

	status := m.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.InProgress {
		t.Error("expected InProgress to be false initially")
	}
}

func TestManager_Scan(t *testing.T) {
	m := NewManager()

	err := m.Scan([]string{"192.168.1.1", "192.168.1.2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Scan is async, status should show in progress
	status := m.GetStatus()
	if !status.InProgress {
		t.Error("expected InProgress to be true during scan")
	}
	if status.Targets != 2 {
		t.Errorf("expected 2 targets, got %d", status.Targets)
	}
}

func TestManager_GetResults(t *testing.T) {
	m := NewManager()

	// Initially empty
	results := m.GetResults()
	if len(results) != 0 {
		t.Errorf("expected 0 results initially, got %d", len(results))
	}
}

func TestManager_PerformScan(t *testing.T) {
	m := NewManager()

	m.mu.Lock()
	m.status = &ScanStatus{InProgress: true, Targets: 1, Scanned: 0}
	m.mu.Unlock()

	m.performScan([]string{"10.0.0.1"})

	m.mu.RLock()
	scanned := m.status.Scanned
	inProgress := m.status.InProgress
	m.mu.RUnlock()

	if scanned != 1 {
		t.Errorf("expected scanned=1, got %d", scanned)
	}
	if inProgress {
		t.Error("expected InProgress to be false after scan")
	}
}

func TestManager_RegisterDevice(t *testing.T) {
	m := NewManager()

	device := &DiscoveredDevice{
		ID:   "test-1",
		Type: "switch",
		Name: "core-switch",
		IP:   "10.0.0.1",
	}

	err := m.RegisterDevice(device)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_GetStatusHTTP(t *testing.T) {
	m := NewManager()

	req := httptest.NewRequest("GET", "/api/discovery/status", nil)
	w := httptest.NewRecorder()

	m.GetStatusHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "in_progress") {
		t.Error("expected response to contain in_progress")
	}
}

func TestManager_ScanHTTP(t *testing.T) {
	m := NewManager()

	body := `{"targets":["192.168.1.1","192.168.1.2"]}`
	req := httptest.NewRequest("POST", "/api/discovery/scan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.ScanHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}
}

func TestScanStatus_Fields(t *testing.T) {
	status := &ScanStatus{
		InProgress: true,
		StartedAt:  time.Now(),
		Targets:    5,
		Scanned:    2,
	}

	if !status.InProgress {
		t.Error("expected InProgress true")
	}
	if status.Targets != 5 {
		t.Errorf("expected Targets 5, got %d", status.Targets)
	}
	if status.Scanned != 2 {
		t.Errorf("expected Scanned 2, got %d", status.Scanned)
	}
}

func TestDiscoveredDevice_Fields(t *testing.T) {
	device := &DiscoveredDevice{
		ID:              "dev-1",
		Type:            "router",
		Name:            "edge-router",
		IP:              "10.0.0.254",
		Port:            22,
		DiscoveryMethod: "ssh",
	}

	if device.ID != "dev-1" {
		t.Errorf("expected ID 'dev-1', got '%s'", device.ID)
	}
	if device.Type != "router" {
		t.Errorf("expected Type 'router', got '%s'", device.Type)
	}
	if device.DiscoveryMethod != "ssh" {
		t.Errorf("expected DiscoveryMethod 'ssh', got '%s'", device.DiscoveryMethod)
	}
}