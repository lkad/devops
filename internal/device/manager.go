package device

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Manager struct {
	repo *Repository
}

type Device struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	Name            string            `json:"name"`
	Status          State             `json:"status"`
	Labels          map[string]string `json:"labels"`
	BusinessUnit    string            `json:"business_unit,omitempty"`
	ComputeCluster  string            `json:"compute_cluster,omitempty"`
	ParentID        string            `json:"parent_id,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	StateHistory    []StateTransition `json:"state_history,omitempty"`
	RegisteredAt    *time.Time        `json:"registered_at,omitempty"`
	LastSeen        *time.Time        `json:"last_seen,omitempty"`
	LastConfigSync  *time.Time        `json:"last_config_sync,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type StateTransition struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Timestamp time.Time `json:"timestamp"`
}

type RegisterOpts struct {
	Type           string
	Name           string
	Labels         map[string]string
	BusinessUnit   string
	ComputeCluster string
	ParentID       string
}

func NewManager(dbDSN string) (*Manager, error) {
	repo, err := NewRepository(dbDSN)
	if err != nil {
		return nil, err
	}
	return &Manager{repo: repo}, nil
}

func (m *Manager) RegisterDevice(opts RegisterOpts) (*Device, error) {
	id := uuid.New().String()
	now := time.Now()

	device := &Device{
		ID:             id,
		Type:           opts.Type,
		Name:           opts.Name,
		Status:         StatePending,
		Labels:         opts.Labels,
		BusinessUnit:   opts.BusinessUnit,
		ComputeCluster: opts.ComputeCluster,
		ParentID:       opts.ParentID,
		Config:         make(map[string]interface{}),
		Metadata:       make(map[string]interface{}),
		RegisteredAt:   &now,
		StateHistory: []StateTransition{
			{From: "", To: StatePending, Timestamp: now},
		},
	}

	if err := m.repo.Create(device); err != nil {
		return nil, err
	}

	return device, nil
}

func (m *Manager) GetDevice(id string) (*Device, error) {
	return m.repo.GetByID(id)
}

func (m *Manager) ListDevices() ([]*Device, error) {
	return m.repo.List()
}

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

	if err := m.repo.Update(device); err != nil {
		return nil, err
	}

	return device, nil
}

func (m *Manager) DeleteDevice(id string) error {
	return m.repo.Delete(id)
}

func (m *Manager) TransitionState(id string, newState State) (*Device, error) {
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

	return device, nil
}

func (m *Manager) SearchDevices(labels map[string]string) ([]*Device, error) {
	return m.repo.SearchByLabels(labels)
}

// HTTP handlers
func (m *Manager) ListDevicesHTTP(w http.ResponseWriter, r *http.Request) {
	devices, err := m.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (m *Manager) CreateDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	var opts RegisterOpts
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	device, err := m.RegisterDevice(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if device == nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (m *Manager) UpdateDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	device, err := m.UpdateDevice(id, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

func (m *Manager) DeleteDeviceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := m.DeleteDevice(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}
