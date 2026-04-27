package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
	"github.com/gosnmp/gosnmp"
)

type Manager struct {
	mu       sync.RWMutex
	status   *ScanStatus
	results  []*DiscoveredDevice
	snmpCfg  *SNMPConfig
}

type SNMPConfig struct {
	Community string
	Timeout   time.Duration
	Retries   int
}

type SNMPDeviceInfo struct {
	IP         string
	SysDescr   string
	SysObjectID string
	SysName    string
	SysContact string
	SysLocation string
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
		snmpCfg: &SNMPConfig{
			Community: "public",
			Timeout:   500 * time.Millisecond,
			Retries:   1,
		},
	}
}

func NewManagerWithSNMP(community string, timeout time.Duration, retries int) *Manager {
	return &Manager{
		status:  &ScanStatus{},
		results: make([]*DiscoveredDevice, 0),
		snmpCfg: &SNMPConfig{
			Community: community,
			Timeout:   timeout,
			Retries:   retries,
		},
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
		// Try SNMP probing first, fall back to simulated discovery
		device := m.snmpProbe(target)

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

func (m *Manager) snmpProbe(target string) *DiscoveredDevice {
	info, err := m.getSNMPDeviceInfo(target)
	if err != nil || info == nil {
		// Fallback to simulated discovery if SNMP fails
		return &DiscoveredDevice{
			ID:              fmt.Sprintf("disc-%d", time.Now().UnixNano()),
			Type:            "network_device",
			Name:            fmt.Sprintf("discovered-%s", target),
			IP:              target,
			Port:            161,
			Labels:          map[string]string{},
			DiscoveryMethod: "snmp_fallback",
		}
	}

	deviceType := m.identifyDeviceType(info)
	deviceName := info.SysName
	if deviceName == "" {
		deviceName = fmt.Sprintf("snmp-%s", target)
	}

	return &DiscoveredDevice{
		ID:              fmt.Sprintf("disc-%d", time.Now().UnixNano()),
		Type:            deviceType,
		Name:            deviceName,
		IP:              info.IP,
		Port:            161,
		Labels:          m.buildSNMPLabels(info),
		DiscoveryMethod: "snmp",
	}
}

func (m *Manager) getSNMPDeviceInfo(target string) (*SNMPDeviceInfo, error) {
	if m.snmpCfg == nil {
		m.snmpCfg = &SNMPConfig{Community: "public", Timeout: 500 * time.Millisecond, Retries: 1}
	}

	// Convert timeout from Duration to seconds (gosnmp expects seconds)
	timeoutSeconds := int64(m.snmpCfg.Timeout.Seconds())
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}

	params, err := gosnmp.NewGoSNMP(target, 161, m.snmpCfg.Community, gosnmp.Version2c, timeoutSeconds)
	if err != nil {
		return nil, fmt.Errorf("failed to create SNMP client for %s: %w", target, err)
	}

	info := &SNMPDeviceInfo{IP: target}

	// OIDs for system info
	oidMap := map[string]*string{
		"1.3.6.1.2.1.1.1.0": &info.SysDescr,     // sysDescr
		"1.3.6.1.2.1.1.2.0": &info.SysObjectID,  // sysObjectID
		"1.3.6.1.2.1.1.5.0": &info.SysName,      // sysName
		"1.3.6.1.2.1.1.4.0": &info.SysContact,   // sysContact
		"1.3.6.1.2.1.1.6.0": &info.SysLocation,  // sysLocation
	}

	for oid, dest := range oidMap {
		result, err := params.Get(oid)
		if err != nil {
			// Continue with other OIDs if one fails
			continue
		}
		if len(result.Variables) > 0 && result.Variables[0].Value != nil {
			*dest = m.parseSNMPValue(result.Variables[0].Value)
		}
	}

	// If we couldn't get any info, return error
	if info.SysDescr == "" && info.SysObjectID == "" && info.SysName == "" {
		return nil, fmt.Errorf("no SNMP response from %s", target)
	}

	return info, nil
}

func (m *Manager) parseSNMPValue(value interface{}) string {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (m *Manager) identifyDeviceType(info *SNMPDeviceInfo) string {
	if info == nil {
		return "unknown"
	}

	// Check sysObjectID patterns for device type identification
	// Enterprise OIDs: .1.3.6.1.4.1.XXX
	objID := strings.ToLower(info.SysObjectID)
	descr := strings.ToLower(info.SysDescr)

	// Normalize OID by stripping leading dot if present
	normalizedOID := objID
	if len(normalizedOID) > 0 && normalizedOID[0] == '.' {
		normalizedOID = normalizedOID[1:]
	}

	// Cisco devices (.1.3.6.1.4.1.9 = ciscoSystems)
	if strings.HasPrefix(normalizedOID, "1.3.6.1.4.1.9.") || strings.Contains(descr, "cisco") {
		if strings.Contains(descr, "router") {
			return "cisco_router"
		}
		if strings.Contains(descr, "switch") || strings.Contains(descr, "catalyst") {
			return "cisco_switch"
		}
		if strings.Contains(descr, "firewall") || strings.Contains(descr, "asa") {
			return "cisco_firewall"
		}
		return "cisco_device"
	}

	// Juniper devices (.1.3.6.1.4.1.2636 = juniperNetworks)
	if strings.HasPrefix(normalizedOID, "1.3.6.1.4.1.2636.") || strings.Contains(descr, "juniper") {
		if strings.Contains(descr, "router") {
			return "juniper_router"
		}
		if strings.Contains(descr, "switch") {
			return "juniper_switch"
		}
		return "juniper_device"
	}

	// HP/Aruba devices (.1.3.6.1.4.1.11 = hp)
	if strings.HasPrefix(normalizedOID, "1.3.6.1.4.1.11.") || strings.HasPrefix(normalizedOID, "1.3.6.1.4.1.14823.") || strings.Contains(descr, "hp ") || strings.Contains(descr, "procurve") || strings.Contains(descr, "aruba") {
		if strings.Contains(descr, "switch") {
			return "hp_switch"
		}
		return "hp_device"
	}

	// Dell devices (.1.3.6.1.4.1.6027 = dell)
	if strings.HasPrefix(normalizedOID, "1.3.6.1.4.1.6027.") || strings.Contains(descr, "dell") {
		if strings.Contains(descr, "switch") {
			return "dell_switch"
		}
		return "dell_device"
	}

	// Ubiquiti
	if strings.Contains(objID, "ubiquiti") || strings.Contains(descr, "ubiquiti") {
		return "ubiquiti_device"
	}

	// Generic network device types based on sysDescr
	if strings.Contains(descr, "router") {
		return "router"
	}
	if strings.Contains(descr, "switch") {
		return "switch"
	}
	if strings.Contains(descr, "firewall") {
		return "firewall"
	}
	if strings.Contains(descr, "access point") || strings.Contains(descr, "wireless") {
		return "wireless_ap"
	}
	if strings.Contains(descr, "print") || strings.Contains(descr, "printer") {
		return "printer"
	}
	if strings.Contains(descr, "server") {
		return "server"
	}

	return "network_device"
}

func (m *Manager) buildSNMPLabels(info *SNMPDeviceInfo) map[string]string {
	labels := make(map[string]string)

	if info.SysDescr != "" {
		labels["snmp_sysdescr"] = info.SysDescr
	}
	if info.SysObjectID != "" {
		labels["snmp_sysObjectID"] = info.SysObjectID
	}
	if info.SysContact != "" {
		labels["snmp_contact"] = info.SysContact
	}
	if info.SysLocation != "" {
		labels["snmp_location"] = info.SysLocation
	}

	return labels
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
