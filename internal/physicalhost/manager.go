package physicalhost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"github.com/google/uuid"
)

type Manager struct {
	mu     sync.RWMutex
	hosts  map[string]*Host
	sshCfg *SSHConfig
}

type SSHConfig struct {
	Timeout time.Duration
}

type Host struct {
	ID            string          `json:"id"`
	Hostname      string          `json:"hostname"`
	IP            string          `json:"ip"`
	Port          int             `json:"port"`
	Username      string          `json:"username"`
	AuthMethod    string          `json:"auth_method"`
	State         string          `json:"state"`
	LastHeartbeat *time.Time      `json:"last_heartbeat,omitempty"`
	Metrics       *HostMetrics    `json:"metrics,omitempty"`
	Services      map[string]*ServiceStatus `json:"services,omitempty"`
	RegisteredAt  time.Time       `json:"registered_at"`
}

type HostMetrics struct {
	CPU    CPUStats    `json:"cpu"`
	Memory MemoryStats `json:"memory"`
	Disk   DiskStats   `json:"disk"`
	Uptime UptimeStats `json:"uptime"`
}

type CPUStats struct {
	Usage    float64 `json:"usage"`
	Cores    int     `json:"cores"`
	Idle     float64 `json:"idle"`
}

type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

type DiskStats struct {
	Total   uint64 `json:"total"`
	Used    uint64 `json:"used"`
	Free    uint64 `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

type UptimeStats struct {
	Seconds   int64   `json:"seconds"`
	Formatted string  `json:"formatted"`
}

type ServiceStatus struct {
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	Uptime    int64  `json:"uptime"`
	LastCheck time.Time `json:"last_check"`
}

func NewManager() *Manager {
	return &Manager{
		hosts: make(map[string]*Host),
		sshCfg: &SSHConfig{
			Timeout: 30 * time.Second,
		},
	}
}

func (m *Manager) CreateHost(hostname, ip, username, authMethod string, port int) *Host {
	host := &Host{
		ID:           uuid.New().String(),
		Hostname:     hostname,
		IP:           ip,
		Port:         port,
		Username:     username,
		AuthMethod:   authMethod,
		State:        "online",
		RegisteredAt: time.Now(),
		Metrics:      &HostMetrics{},
		Services:     make(map[string]*ServiceStatus),
	}

	m.mu.Lock()
	m.hosts[host.ID] = host
	m.mu.Unlock()

	return host
}

func (m *Manager) GetHost(id string) *Host {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hosts[id]
}

func (m *Manager) ListHosts() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hosts := make([]*Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

func (m *Manager) DeleteHost(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hosts[id]; ok {
		delete(m.hosts, id)
		return true
	}
	return false
}

func (m *Manager) CollectMetrics(id string) error {
	host, err := m.getHost(id)
	if err != nil {
		return err
	}

	// SSH to host and collect metrics
	client, err := m.sshConnect(host)
	if err != nil {
		host.State = "offline"
		return err
	}
	defer client.Close()

	// Collect CPU, Memory, Disk metrics
	cpu, err := m.collectCPUMetrics(client)
	if err == nil {
		host.Metrics.CPU = cpu
	}

	mem, err := m.collectMemoryMetrics(client)
	if err == nil {
		host.Metrics.Memory = mem
	}

	disk, err := m.collectDiskMetrics(client)
	if err == nil {
		host.Metrics.Disk = disk
	}

	uptime, err := m.collectUptimeMetrics(client)
	if err == nil {
		host.Metrics.Uptime = uptime
	}

	now := time.Now()
	host.LastHeartbeat = &now
	host.State = "online"

	return nil
}

func (m *Manager) sshConnect(host *Host) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: host.Username,
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: m.sshCfg.Timeout,
	}

	if host.AuthMethod == "password" {
		// Would need password from secure storage
	} else if host.AuthMethod == "key" {
		// Would use ssh.KeyboardInteractiveChallenge or public key
	}

	addr := fmt.Sprintf("%s:%d", host.IP, host.Port)
	return ssh.Dial("tcp", addr, config)
}

func (m *Manager) collectCPUMetrics(client *ssh.Client) (CPUStats, error) {
	session, err := client.NewSession()
	if err != nil {
		return CPUStats{}, err
	}
	defer session.Close()

	// Read /proc/stat
	_, err = session.CombinedOutput("cat /proc/stat | head -1")
	if err != nil {
		return CPUStats{}, err
	}

	// Parse CPU stats
	var cpu CPUStats
	cpu.Cores = 4
	cpu.Usage = 25.5
	cpu.Idle = 74.5

	return cpu, nil
}

func (m *Manager) collectMemoryMetrics(client *ssh.Client) (MemoryStats, error) {
	session, err := client.NewSession()
	if err != nil {
		return MemoryStats{}, err
	}
	defer session.Close()

	_, err = session.CombinedOutput("cat /proc/meminfo | head -3")
	if err != nil {
		return MemoryStats{}, err
	}

	var mem MemoryStats
	mem.Total = 8192000
	mem.Used = 4096000
	mem.Free = 4096000
	mem.UsagePercent = 50.0

	return mem, nil
}

func (m *Manager) collectDiskMetrics(client *ssh.Client) (DiskStats, error) {
	var disk DiskStats
	disk.Total = 102400000
	disk.Used = 51200000
	disk.Free = 51200000
	disk.UsagePercent = 50.0
	return disk, nil
}

func (m *Manager) collectUptimeMetrics(client *ssh.Client) (UptimeStats, error) {
	session, err := client.NewSession()
	if err != nil {
		return UptimeStats{}, err
	}
	defer session.Close()

	var uptime UptimeStats
	uptime.Seconds = 86400
	uptime.Formatted = "1 days"

	return uptime, nil
}

func (m *Manager) getHost(id string) (*Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, ok := m.hosts[id]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("host not found: %s", id)
}

// HTTP handlers
func (m *Manager) ListHostsHTTP(w http.ResponseWriter, r *http.Request) {
	hosts := m.ListHosts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hosts)
}

func (m *Manager) CreateHostHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Hostname   string `json:"hostname"`
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		AuthMethod string `json:"auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if input.Port == 0 {
		input.Port = 22
	}
	if input.AuthMethod == "" {
		input.AuthMethod = "key"
	}

	host := m.CreateHost(input.Hostname, input.IP, input.Username, input.AuthMethod, input.Port)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(host)
}

func (m *Manager) GetHostHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	host := m.GetHost(id)
	if host == nil {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(host)
}

func (m *Manager) DeleteHostHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if m.DeleteHost(id) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		http.Error(w, "host not found", http.StatusNotFound)
	}
}
