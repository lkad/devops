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

func TestSNMPDeviceInfo_Fields(t *testing.T) {
	info := &SNMPDeviceInfo{
		IP:          "192.168.1.1",
		SysDescr:    "Cisco IOS Software, Version 15.2",
		SysObjectID: ".1.3.6.1.4.1.9.1.1",
		SysName:     "core-router-01",
		SysContact:  "admin@example.com",
		SysLocation: "Data Center A",
	}

	if info.IP != "192.168.1.1" {
		t.Errorf("expected IP '192.168.1.1', got '%s'", info.IP)
	}
	if info.SysDescr == "" {
		t.Error("expected SysDescr to be set")
	}
	if info.SysObjectID == "" {
		t.Error("expected SysObjectID to be set")
	}
}

func TestSNMPConfig_Defaults(t *testing.T) {
	cfg := &SNMPConfig{
		Community: "public",
		Timeout:   5 * time.Second,
		Retries:   2,
	}

	if cfg.Community != "public" {
		t.Errorf("expected Community 'public', got '%s'", cfg.Community)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected Timeout 5s, got %v", cfg.Timeout)
	}
}

func TestNewManagerWithSNMP(t *testing.T) {
	m := NewManagerWithSNMP("private", 10*time.Second, 3)

	if m.snmpCfg == nil {
		t.Fatal("expected snmpCfg to be initialized")
	}
	if m.snmpCfg.Community != "private" {
		t.Errorf("expected Community 'private', got '%s'", m.snmpCfg.Community)
	}
	if m.snmpCfg.Timeout != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", m.snmpCfg.Timeout)
	}
	if m.snmpCfg.Retries != 3 {
		t.Errorf("expected Retries 3, got %d", m.snmpCfg.Retries)
	}
}

func TestNewManager_HasDefaultSNMPConfig(t *testing.T) {
	m := NewManager()

	if m.snmpCfg == nil {
		t.Fatal("expected snmpCfg to be initialized with defaults")
	}
	if m.snmpCfg.Community != "public" {
		t.Errorf("expected default Community 'public', got '%s'", m.snmpCfg.Community)
	}
}

func TestIdentifyDeviceType_Cisco(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		descr     string
		objID     string
		expected  string
	}{
		{"Cisco IOS Router", "", "cisco_router"},
		{"Cisco Catalyst Switch", "", "cisco_switch"},
		{"Cisco ASA Firewall", "", "cisco_firewall"},
		{"", ".1.3.6.1.4.1.9.1.1", "cisco_device"},
	}

	for _, tt := range tests {
		info := &SNMPDeviceInfo{SysDescr: tt.descr, SysObjectID: tt.objID}
		result := m.identifyDeviceType(info)
		if result != tt.expected {
			t.Errorf("identifyDeviceType(%q, %q) = %q, want %q", tt.descr, tt.objID, result, tt.expected)
		}
	}
}

func TestIdentifyDeviceType_Juniper(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		descr     string
		objID     string
		expected  string
	}{
		{"Juniper JUNOS Router", "", "juniper_router"},
		{"Juniper EX Switch", "", "juniper_switch"},
		{"", ".1.3.6.1.4.1.2636.1.1", "juniper_device"},
	}

	for _, tt := range tests {
		info := &SNMPDeviceInfo{SysDescr: tt.descr, SysObjectID: tt.objID}
		result := m.identifyDeviceType(info)
		if result != tt.expected {
			t.Errorf("identifyDeviceType(%q, %q) = %q, want %q", tt.descr, tt.objID, result, tt.expected)
		}
	}
}

func TestIdentifyDeviceType_HP(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		descr     string
		objID     string
		expected  string
	}{
		{"HP ProCurve Switch", "", "hp_switch"},
		{"HP J4813A Switch", "", "hp_switch"},
		{"HP Printer", "", "hp_device"},
		{"", ".1.3.6.1.4.1.11.2.3.4.5", "hp_device"},
	}

	for _, tt := range tests {
		info := &SNMPDeviceInfo{SysDescr: tt.descr, SysObjectID: tt.objID}
		result := m.identifyDeviceType(info)
		if result != tt.expected {
			t.Errorf("identifyDeviceType(%q, %q) = %q, want %q", tt.descr, tt.objID, result, tt.expected)
		}
	}
}

func TestIdentifyDeviceType_Dell(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		descr     string
		objID     string
		expected  string
	}{
		{"Dell PowerConnect Switch", "", "dell_switch"},
		{"Dell Networking Switch", "", "dell_switch"},
		{"Dell Server", "", "dell_device"},
		{"", ".1.3.6.1.4.1.6027.1.1", "dell_device"},
	}

	for _, tt := range tests {
		info := &SNMPDeviceInfo{SysDescr: tt.descr, SysObjectID: tt.objID}
		result := m.identifyDeviceType(info)
		if result != tt.expected {
			t.Errorf("identifyDeviceType(%q, %q) = %q, want %q", tt.descr, tt.objID, result, tt.expected)
		}
	}
}

func TestIdentifyDeviceType_Generic(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		descr     string
		objID     string
		expected  string
	}{
		{"Generic Router Device", "", "router"},
		{"Layer 3 Switch", "", "switch"},
		{"Firewall Appliance", "", "firewall"},
		{"Wireless Access Point", "", "wireless_ap"},
		{"Network Printer", "", "printer"},
	}

	for _, tt := range tests {
		info := &SNMPDeviceInfo{SysDescr: tt.descr, SysObjectID: tt.objID}
		result := m.identifyDeviceType(info)
		if result != tt.expected {
			t.Errorf("identifyDeviceType(%q, %q) = %q, want %q", tt.descr, tt.objID, result, tt.expected)
		}
	}
}

func TestIdentifyDeviceType_Nil(t *testing.T) {
	m := &Manager{}
	result := m.identifyDeviceType(nil)
	if result != "unknown" {
		t.Errorf("identifyDeviceType(nil) = %q, want %q", result, "unknown")
	}
}

func TestBuildSNMPLabels(t *testing.T) {
	m := &Manager{}

	info := &SNMPDeviceInfo{
		IP:          "192.168.1.1",
		SysDescr:    "Cisco IOS Router",
		SysObjectID: ".1.3.6.1.4.1.9.1.1",
		SysName:     "router-01",
		SysContact:  "admin@example.com",
		SysLocation: "Data Center",
	}

	labels := m.buildSNMPLabels(info)

	if labels["snmp_sysdescr"] != info.SysDescr {
		t.Errorf("expected snmp_sysdescr %q, got %q", info.SysDescr, labels["snmp_sysdescr"])
	}
	if labels["snmp_sysObjectID"] != info.SysObjectID {
		t.Errorf("expected snmp_sysObjectID %q, got %q", info.SysObjectID, labels["snmp_sysObjectID"])
	}
	if labels["snmp_contact"] != info.SysContact {
		t.Errorf("expected snmp_contact %q, got %q", info.SysContact, labels["snmp_contact"])
	}
	if labels["snmp_location"] != info.SysLocation {
		t.Errorf("expected snmp_location %q, got %q", info.SysLocation, labels["snmp_location"])
	}
}

func TestParseSNMPValue(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		input    interface{}
		expected string
	}{
		{[]byte("test string"), "test string"},
		{"direct string", "direct string"},
		{int(42), "42"},
	}

	for _, tt := range tests {
		result := m.parseSNMPValue(tt.input)
		if result != tt.expected {
			t.Errorf("parseSNMPValue(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

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