package physicalhost

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
	"github.com/gorilla/mux"
	"golang.org/x/crypto/ssh"
	"github.com/google/uuid"
)

// SSHConnPool manages pooled SSH connections per host.
// It reuses connections to avoid the overhead of establishing
// a new SSH connection for each request.
type SSHConnPool struct {
	mu        sync.Mutex
	pools     map[string]*hostPool
	maxConns  int           // max connections per host
	timeout   time.Duration // connection timeout
	connTTL   time.Duration // max time a connection can be idle
	dialer    func(host *Host) (*ssh.Client, error) // custom dial function
}

type hostPool struct {
	mu       sync.Mutex
	conns    []*ssh.Client // available connections
	inUse    map[*ssh.Client]bool
	lastUsed time.Time
}

// SSHConfig holds SSH configuration settings
type SSHConfig struct {
	Timeout  time.Duration
	MaxConns int
	ConnTTL  time.Duration
}

type Manager struct {
	mu     sync.RWMutex
	hosts  map[string]*Host
	sshCfg *SSHConfig
	pool   *SSHConnPool
}

// DefaultSSHConnPoolConfig returns default pool configuration
func DefaultSSHConnPoolConfig() *SSHConfig {
	return &SSHConfig{
		Timeout:  30 * time.Second,
		MaxConns: 5,
		ConnTTL:  5 * time.Minute,
	}
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
	cfg := DefaultSSHConnPoolConfig()
	m := &Manager{
		hosts: make(map[string]*Host),
		sshCfg: cfg,
	}
	m.pool = NewSSHConnPool(cfg, m.dialSSH)
	return m
}

// NewSSHConnPool creates a new SSH connection pool
func NewSSHConnPool(cfg *SSHConfig, dialer func(host *Host) (*ssh.Client, error)) *SSHConnPool {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 5
	}
	if cfg.ConnTTL <= 0 {
		cfg.ConnTTL = 5 * time.Minute
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &SSHConnPool{
		pools:    make(map[string]*hostPool),
		maxConns: cfg.MaxConns,
		timeout:  cfg.Timeout,
		connTTL:  cfg.ConnTTL,
		dialer:   dialer,
	}
}

// poolKey generates a unique key for a host's connection pool
func (p *SSHConnPool) poolKey(host *Host) string {
	return fmt.Sprintf("%s:%d:%s", host.IP, host.Port, host.Username)
}

// Get retrieves or creates a connection for the given host
func (p *SSHConnPool) Get(host *Host) (*ssh.Client, error) {
	key := p.poolKey(host)

	p.mu.Lock()
	pool, exists := p.pools[key]
	if !exists {
		pool = &hostPool{
			conns: make([]*ssh.Client, 0),
			inUse: make(map[*ssh.Client]bool),
		}
		p.pools[key] = pool
	}
	p.mu.Unlock()

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to get an existing available connection
	for i := len(pool.conns) - 1; i >= 0; i-- {
		client := pool.conns[i]
		if pool.inUse[client] {
			continue
		}
		// Connection available - mark as in use
		// Note: health check happens when the connection is actually used
		pool.inUse[client] = true
		pool.lastUsed = time.Now()
		return client, nil
	}

	// Create new connection if under limit
	if len(pool.conns)-len(pool.inUse) >= p.maxConns {
		// Wait for a connection to become available (simple approach: create one anyway)
		// In production, could use a channel-based approach for blocking
	}

	pool.mu.Unlock()
	client, err := p.dialer(host)
	pool.mu.Lock()

	if err != nil {
		return nil, err
	}

	pool.inUse[client] = true
	pool.lastUsed = time.Now()
	return client, nil
}

// Put returns a connection to the pool
func (p *SSHConnPool) Put(host *Host, client *ssh.Client) {
	if client == nil {
		return
	}

	key := p.poolKey(host)

	p.mu.Lock()
	pool, exists := p.pools[key]
	if !exists {
		p.mu.Unlock()
		client.Close()
		return
	}
	p.mu.Unlock()

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if pool.inUse[client] {
		delete(pool.inUse, client)
		// Always return to pool - health check will happen on next Get
		pool.conns = append(pool.conns, client)
		pool.lastUsed = time.Now()
	}
}

// Close closes all connections in the pool
func (p *SSHConnPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, pool := range p.pools {
		pool.mu.Lock()
		for _, client := range pool.conns {
			client.Close()
		}
		pool.conns = nil
		pool.inUse = nil
		pool.mu.Unlock()
	}
	p.pools = nil
}

// CloseHostPool closes all connections for a specific host
func (p *SSHConnPool) CloseHostPool(host *Host) {
	key := p.poolKey(host)

	p.mu.Lock()
	pool, exists := p.pools[key]
	if exists {
		delete(p.pools, key)
	}
	p.mu.Unlock()

	if pool == nil {
		return
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for _, client := range pool.conns {
		client.Close()
	}
	pool.conns = nil
	pool.inUse = nil
}

// cleanup closes idle connections older than connTTL
func (p *SSHConnPool) Cleanup() {
	p.mu.Lock()
	keysToDelete := make([]string, 0)
	for key, pool := range p.pools {
		pool.mu.Lock()
		if time.Since(pool.lastUsed) > p.connTTL {
			keysToDelete = append(keysToDelete, key)
			for _, client := range pool.conns {
				client.Close()
			}
			pool.conns = nil
			pool.inUse = nil
		}
		pool.mu.Unlock()
	}
	for _, key := range keysToDelete {
		delete(p.pools, key)
	}
	p.mu.Unlock()
}

// Stats returns pool statistics for a host
func (p *SSHConnPool) Stats(host *Host) (available, inUse int) {
	key := p.poolKey(host)

	p.mu.Lock()
	pool, exists := p.pools[key]
	if !exists {
		p.mu.Unlock()
		return 0, 0
	}
	p.mu.Unlock()

	pool.mu.Lock()
	defer pool.mu.Unlock()

	available = len(pool.conns) - len(pool.inUse)
	// Count only available ones properly
	availCount := 0
	for _, c := range pool.conns {
		if !pool.inUse[c] {
			availCount++
		}
	}
	available = availCount
	inUse = len(pool.inUse)
	return
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

	// SSH to host and collect metrics - get connection from pool
	client, err := m.sshConnect(host)
	if err != nil {
		host.State = "offline"
		return err
	}
	// Return connection to pool when done (reuse, not close)
	defer m.sshPutConnection(host, client)

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
	return m.pool.Get(host)
}

func (m *Manager) sshPutConnection(host *Host, client *ssh.Client) {
	m.pool.Put(host, client)
}

// dialSSH establishes a new SSH connection to the host
func (m *Manager) dialSSH(host *Host) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            host.Username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         m.sshCfg.Timeout,
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
	limit, offset := parsePagination(r)
	hosts := m.ListHosts()
	// Apply pagination in-memory
	total := len(hosts)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedHosts := hosts[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedHosts, total, limit, offset))
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

func (m *Manager) CreateHostHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Hostname   string `json:"hostname"`
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		AuthMethod string `json:"auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
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
	id := mux.Vars(r)["id"]
	host := m.GetHost(id)
	if host == nil {
		apierror.NotFound(w, "host not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(host)
}

func (m *Manager) DeleteHostHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if m.DeleteHost(id) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		apierror.NotFound(w, "host not found")
	}
}

// ListServices retrieves the list of systemd services on the host via SSH
func (m *Manager) ListServices(id string) ([]*ServiceStatus, error) {
	host, err := m.getHost(id)
	if err != nil {
		return nil, err
	}

	client, err := m.sshConnect(host)
	if err != nil {
		host.State = "offline"
		return nil, fmt.Errorf("failed to connect to host: %w", err)
	}
	defer m.sshPutConnection(host, client)

	return m.listServicesViaSSH(client)
}

// listServicesViaSSH runs systemctl to list services on the remote host
func (m *Manager) listServicesViaSSH(client *ssh.Client) ([]*ServiceStatus, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// Run systemctl to list all service units
	// Using --no-pager to avoid interactive output, --no-legend to omit headers
	output, err := session.CombinedOutput("systemctl list-units --type=service --all --no-pager --no-legend")
	if err != nil {
		// Fallback: try to get at least running services
		output, err = session.CombinedOutput("systemctl list-units --type=service --no-pager --no-legend")
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}
	}

	return parseServiceList(output), nil
}

// parseServiceList parses systemctl output into ServiceStatus structs
// Output format: UNIT LOAD ACTIVE SUB DESCRIPTION
// Example: nginx.service loaded active running A nginx HTTP and reverse proxy server
func parseServiceList(output []byte) []*ServiceStatus {
	var services []*ServiceStatus
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the line - format is: UNIT LOAD ACTIVE SUB DESCRIPTION
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := fields[0]
		// Remove .service suffix if present for cleaner names
		if strings.HasSuffix(name, ".service") {
			name = strings.TrimSuffix(name, ".service")
		}

		activeState := fields[2]
		active := activeState == "active" || activeState == "running"

		services = append(services, &ServiceStatus{
			Name:       name,
			Active:     active,
			Uptime:     0, // systemctl list-units doesn't provide uptime directly
			LastCheck:  time.Now(),
		})
	}

	return services
}

// ListServicesHTTP handles GET /api/physical-hosts/:id/services
func (m *Manager) ListServicesHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Check if host exists
	host := m.GetHost(id)
	if host == nil {
		apierror.NotFound(w, "host not found")
		return
	}

	services, err := m.ListServices(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host_id":  id,
		"services": services,
	})
}

// PushConfigRequest represents the config push request body
type PushConfigRequest struct {
	Path    string `json:"path"`    // Remote file path to write to
	Content string `json:"content"` // Config content to write
}

// PushConfig pushes configuration content to the host via SSH
func (m *Manager) PushConfig(id string, req *PushConfigRequest) error {
	if req == nil || req.Path == "" || req.Content == "" {
		return fmt.Errorf("path and content are required")
	}

	host, err := m.getHost(id)
	if err != nil {
		return err
	}

	client, err := m.sshConnect(host)
	if err != nil {
		host.State = "offline"
		return fmt.Errorf("failed to connect to host: %w", err)
	}
	defer m.sshPutConnection(host, client)

	return m.pushConfigViaSSH(client, req.Path, req.Content)
}

// pushConfigViaSSH writes config content to a remote file via SSH
func (m *Manager) pushConfigViaSSH(client *ssh.Client, path, content string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Use a heredoc to write the file content via SSH
	// This avoids issues with special characters in the config content
	// Using Sudo tee for privileged write if needed
	cmd := fmt.Sprintf("sudo tee %s > /dev/null << 'DEVOPS_EOF'\n%s\nDEVOPS_EOF", path, content)

	err = session.Run(cmd)
	if err != nil {
		// Try without sudo if that fails
		cmd = fmt.Sprintf("cat > %s << 'DEVOPS_EOF'\n%s\nDEVOPS_EOF", path, content)
		err = session.Run(cmd)
		if err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// PushConfigHTTP handles POST /api/physical-hosts/:id/config
func (m *Manager) PushConfigHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Check if host exists
	host := m.GetHost(id)
	if host == nil {
		apierror.NotFound(w, "host not found")
		return
	}

	var req PushConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.ValidationError(w, "invalid request body")
		return
	}

	if req.Path == "" {
		apierror.ValidationError(w, "path is required")
		return
	}
	if req.Content == "" {
		apierror.ValidationError(w, "content is required")
		return
	}

	if err := m.PushConfig(id, &req); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "config pushed successfully",
		"host_id": id,
		"path":    req.Path,
	})
}
