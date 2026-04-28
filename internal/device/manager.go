package device

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// Manager handles device operations
type Manager struct {
	repo        *Repository
	hypervisor HypervisorClient
	metrics    MetricsCollector
	network    NetworkDeviceClient
}

// Environment represents the deployment environment
type Environment string

const (
	EnvProd Environment = "prod"
	EnvDev  Environment = "dev"
	EnvTest Environment = "test"
)

// Device is the domain model
type Device struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Name            string                 `json:"name"`
	Status          State                  `json:"status"`
	Environment     Environment             `json:"environment"`
	Labels          map[string]string      `json:"labels"`
	BusinessUnit    string                 `json:"business_unit,omitempty"`
	ComputeCluster  string                 `json:"compute_cluster,omitempty"`
	ParentID        string                 `json:"parent_id,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	StateHistory    []StateTransition      `json:"state_history,omitempty"`
	RegisteredAt    *time.Time             `json:"registered_at,omitempty"`
	LastSeen        *time.Time             `json:"last_seen,omitempty"`
	LastConfigSync  *time.Time             `json:"last_config_sync,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// StateTransition record
type StateTransition struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Timestamp time.Time `json:"timestamp"`
}

// RegisterOpts for device registration
type RegisterOpts struct {
	Type           string
	Name           string
	Environment    Environment
	Labels         map[string]string
	BusinessUnit   string
	ComputeCluster string
	ParentID       string
}

// NewManager creates a new device manager
func NewManager(db *gorm.DB) *Manager {
	repo := NewRepository(db)
	return &Manager{repo: repo}
}

// NewManagerWithClients creates manager with external service clients
func NewManagerWithClients(db *gorm.DB, h HypervisorClient, m MetricsCollector, n NetworkDeviceClient) *Manager {
	return &Manager{
		repo:        NewRepository(db),
		hypervisor: h,
		metrics:    m,
		network:    n,
	}
}

// RegisterDevice creates a new device
func (m *Manager) RegisterDevice(opts RegisterOpts) (*Device, error) {
	id := uuid.New().String()
	now := time.Now()

	env := opts.Environment
	if env == "" {
		env = EnvDev
	}

	device := &Device{
		ID:           id,
		Type:         opts.Type,
		Name:         opts.Name,
		Status:       StatePending,
		Environment:  env,
		Labels:       opts.Labels,
		BusinessUnit: opts.BusinessUnit,
		ComputeCluster: opts.ComputeCluster,
		ParentID:     opts.ParentID,
		Config:       make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
		RegisteredAt: &now,
		StateHistory: []StateTransition{
			{From: "", To: StatePending, Timestamp: now},
		},
	}

	if err := m.repo.Create(device); err != nil {
		return nil, err
	}

	return device, nil
}

// GetDevice retrieves a device by ID
func (m *Manager) GetDevice(id string) (*Device, error) {
	return m.repo.GetByID(id)
}

// ListDevices returns all devices
func (m *Manager) ListDevices() ([]*Device, error) {
	return m.repo.List()
}

// ListDevicesPaginated returns devices with pagination
func (m *Manager) ListDevicesPaginated(limit, offset int) ([]*Device, int, error) {
	return m.repo.ListPaginated(limit, offset)
}

// ListDevicesByType returns devices of a specific type
func (m *Manager) ListDevicesByType(deviceType string, limit, offset int) ([]*Device, int, error) {
	return m.repo.ListByType(deviceType, limit, offset)
}

// UpdateDevice updates device properties
func (m *Manager) UpdateDevice(id string, updates map[string]interface{}) (*Device, error) {
	device, err := m.repo.GetByID(id)
	if err != nil || device == nil {
		return nil, err
	}

	if name, ok := updates["name"].(string); ok {
		device.Name = name
	}
	if labels, ok := updates["labels"].(map[string]string); ok {
		device.Labels = labels
	}
	if bu, ok := updates["business_unit"].(string); ok {
		device.BusinessUnit = bu
	}
	if cc, ok := updates["compute_cluster"].(string); ok {
		device.ComputeCluster = cc
	}
	if env, ok := updates["environment"].(string); ok {
		device.Environment = Environment(env)
	}
	if config, ok := updates["config"].(map[string]interface{}); ok {
		device.Config = config
	}

	if err := m.repo.Update(device); err != nil {
		return nil, err
	}

	return device, nil
}

// DeleteDevice removes a device
func (m *Manager) DeleteDevice(id string) error {
	return m.repo.Delete(id)
}

// TransitionState changes device state with validation
func (m *Manager) TransitionState(id string, newState State, triggeredBy string, reason string) (*Device, error) {
	device, err := m.repo.GetByID(id)
	if err != nil || device == nil {
		return nil, err
	}

	if !device.Status.CanTransitionTo(newState) {
		return nil, fmt.Errorf("invalid transition from %s to %s", device.Status, newState)
	}

	oldState := device.Status
	device.Status = newState
	device.StateHistory = append(device.StateHistory, StateTransition{
		From:      oldState,
		To:        newState,
		Timestamp: time.Now(),
	})

	if err := m.repo.Update(device); err != nil {
		return nil, err
	}

	// Log state transition
	m.repo.RecordStateTransition(id, oldState, newState, triggeredBy, reason)

	return device, nil
}

// SearchDevices finds devices by labels
func (m *Manager) SearchDevices(labels map[string]string) ([]*Device, error) {
	return m.repo.SearchByLabels(labels)
}

// DiscoverVMsFromHost discovers VMs running on a physical host
func (m *Manager) DiscoverVMsFromHost(ctx context.Context, hostID string) ([]*Device, error) {
	if m.hypervisor == nil {
		return nil, fmt.Errorf("hypervisor client not configured")
	}

	vms, err := m.hypervisor.ListVMs(ctx, hostID)
	if err != nil {
		return nil, err
	}

	var devices []*Device
	for _, vm := range vms {
		// Convert VM to Device
		config := VMConfig{
			VMID:           vm.ID,
			HypervisorHost: vm.Hypervisor,
			VCPU:           vm.VCPU,
			MemoryMB:       vm.MemoryMB,
			DiskTotalGB:    vm.DiskGB,
			IPAddresses:    vm.IPAddresses,
			MACAddress:     vm.MACAddress,
			GuestOS:        vm.GuestOS,
			Cluster:        vm.Cluster,
			PowerState:     vm.State,
		}

		configJSON, _ := json.Marshal(config)
		var configMap map[string]interface{}
		json.Unmarshal(configJSON, &configMap)

		device := &Device{
			ID:          vm.ID,
			Type:        string(TypeVM),
			Name:        vm.Name,
			Status:      StateRunning,
			Labels:      map[string]string{"discovered": "true"},
			Config:      configMap,
			Environment: EnvProd,
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// GetVMMetrics retrieves metrics for a VM
func (m *Manager) GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error) {
	if m.metrics == nil {
		return nil, fmt.Errorf("metrics collector not configured")
	}
	return m.metrics.CollectVMMetrics(ctx, vmID)
}

// GetHostMetrics retrieves metrics for a physical host
func (m *Manager) GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error) {
	if m.metrics == nil {
		return nil, fmt.Errorf("metrics collector not configured")
	}
	return m.metrics.CollectHostMetrics(ctx, hostID)
}

// GetNetworkDeviceInterfaces retrieves interfaces for a network device
func (m *Manager) GetNetworkDeviceInterfaces(ctx context.Context, deviceID string) ([]*NetworkInterface, error) {
	if m.network == nil {
		return nil, fmt.Errorf("network device client not configured")
	}
	return m.network.GetDeviceInterfaces(ctx, deviceID)
}

// GetNetworkDeviceMetrics retrieves metrics for a network device
func (m *Manager) GetNetworkDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error) {
	if m.metrics == nil {
		return nil, fmt.Errorf("metrics collector not configured")
	}
	return m.metrics.CollectNetworkDeviceMetrics(ctx, deviceID)
}

// BackupNetworkDeviceConfig triggers config backup for a network device
func (m *Manager) BackupNetworkDeviceConfig(ctx context.Context, deviceID string) (string, error) {
	if m.network == nil {
		return "", fmt.Errorf("network device client not configured")
	}
	return m.network.BackupConfig(ctx, deviceID)
}

// VMPowerControl controls VM power state
func (m *Manager) VMPowerControl(ctx context.Context, vmID string, action string) error {
	if m.hypervisor == nil {
		return fmt.Errorf("hypervisor client not configured")
	}

	switch action {
	case "power-on":
		// For power on, we might need to track in metadata
		return nil
	case "power-off":
		return nil
	case "suspend":
		return nil
	case "resume":
		return nil
	default:
		return fmt.Errorf("unknown power action: %s", action)
	}
}

// HTTP handlers

func (m *Manager) ListDevicesHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	devices, total, err := m.ListDevicesPaginated(limit, offset)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(devices, total, limit, offset))
}

func (m *Manager) CreateDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	var opts RegisterOpts
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	device, err := m.RegisterDevice(opts)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(device)
}

func (m *Manager) GetDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	device, err := m.GetDevice(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if device == nil {
		apierror.NotFound(w, "device not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (m *Manager) UpdateDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	device, err := m.UpdateDevice(id, updates)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (m *Manager) DeleteDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := m.DeleteDevice(id); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *Manager) SearchDevicesHTTP(w http.ResponseWriter, r *http.Request) {
	labels := make(map[string]string)
	query := r.URL.Query()
	for k, v := range query {
		if len(v) > 0 {
			labels[k] = v[0]
		}
	}

	devices, err := m.SearchDevices(labels)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (m *Manager) TransitionStateHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		State       string `json:"state"`
		TriggeredBy string `json:"triggered_by"`
		Reason      string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	device, err := m.TransitionState(id, State(body.State), body.TriggeredBy, body.Reason)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

// VM-specific handlers

func (m *Manager) DiscoverVMsHTTP(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host_id")
	ctx := context.Background()

	devices, err := m.DiscoverVMsFromHost(ctx, hostID)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (m *Manager) GetVMMetricsHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := context.Background()

	metrics, err := m.GetVMMetrics(ctx, id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (m *Manager) VMPowerHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	ctx := context.Background()
	if err := m.VMPowerControl(ctx, id, body.Action); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Network device handlers

func (m *Manager) GetNetworkInterfacesHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := context.Background()

	interfaces, err := m.GetNetworkDeviceInterfaces(ctx, id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interfaces)
}

func (m *Manager) GetNetworkMetricsHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := context.Background()

	metrics, err := m.GetNetworkDeviceMetrics(ctx, id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (m *Manager) BackupConfigHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := context.Background()

	result, err := m.BackupNetworkDeviceConfig(ctx, id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": result})
}

// Physical host handlers

func (m *Manager) GetHostMetricsHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := context.Background()

	metrics, err := m.GetHostMetrics(ctx, id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
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