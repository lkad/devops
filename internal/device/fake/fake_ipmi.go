package fake

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/device"
)

// FakeIPMIClient simulates IPMI/BMC responses for physical host management
type FakeIPMIClient struct {
	mu    sync.RWMutex
	Hosts []*device.GORMDevice
}

// NewFakeIPMIClient creates a new fake IPMI client
func NewFakeIPMIClient() *FakeIPMIClient {
	return &FakeIPMIClient{
		Hosts: []*device.GORMDevice{
			{
				ID:   "bmc-host-001",
				Name: "dell-r740-01",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer":  "Dell",
					"model":         "PowerEdge R740",
					"serial_no":     "SNABC123",
					"cpu_cores":     32,
					"memory_gb":     128,
					"mgmt_ip":       "192.168.1.101",
					"ipmi_ip":       "192.168.1.100",
					"power_watts":   450,
					"temp_celsius":  38,
				},
			},
			{
				ID:   "bmc-host-002",
				Name: "hp-dl380-01",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer":  "HP",
					"model":         "ProLiant DL380 Gen10",
					"serial_no":     "SNHP456789",
					"cpu_cores":     64,
					"memory_gb":     256,
					"mgmt_ip":       "192.168.1.102",
					"ipmi_ip":       "192.168.1.101",
					"power_watts":   550,
					"temp_celsius":  35,
				},
			},
			{
				ID:   "bmc-host-003",
				Name: "lenovo-sr650-01",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer":  "Lenovo",
					"model":         "ThinkSystem SR650",
					"serial_no":     "SNLNV789012",
					"cpu_cores":     24,
					"memory_gb":     64,
					"mgmt_ip":       "192.168.1.103",
					"ipmi_ip":       "192.168.1.102",
					"power_watts":   350,
					"temp_celsius":  40,
				},
			},
		},
	}
}

// ListVMs is not applicable for IPMI, returns empty
func (f *FakeIPMIClient) ListVMs(ctx context.Context, hostID string) ([]*device.VM, error) {
	return []*device.VM{}, nil
}

// GetVM is not applicable for IPMI
func (f *FakeIPMIClient) GetVM(ctx context.Context, vmID string) (*device.VM, error) {
	return nil, errors.New("IPMI does not support VM operations")
}

// GetVMMetrics is not applicable for IPMI
func (f *FakeIPMIClient) GetVMMetrics(ctx context.Context, vmID string) (*device.VMMetrics, error) {
	return nil, errors.New("IPMI does not support VM metrics")
}

// GetHostInfo returns information about a physical host
func (f *FakeIPMIClient) GetHostInfo(ctx context.Context, hostID string) (*device.GORMDevice, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			return host, nil
		}
	}
	return nil, errors.New("host not found")
}

// GetHostMetrics returns BMC metrics for a physical host
func (f *FakeIPMIClient) GetHostMetrics(ctx context.Context, hostID string) (*device.HostMetrics, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			// Get base values from config
			powerWatts := 450
			tempCelsius := 38
			if v, ok := host.Config["power_watts"].(float64); ok {
				powerWatts = int(v)
			}
			if v, ok := host.Config["temp_celsius"].(float64); ok {
				tempCelsius = int(v)
			}

			rand.Seed(time.Now().UnixNano() + int64(len(hostID))*2000)
			return &device.HostMetrics{
				HostID:           hostID,
				CPUUsage:         30 + rand.Float64()*40,
				MemoryUsage:      50 + rand.Float64()*35,
				MemoryTotalGB:    128,
				MemoryUsedGB:     64 + rand.Intn(32),
				DiskUsagePercent: 45 + rand.Float64()*30,
				PowerWatts:       powerWatts + rand.Intn(50) - 25,
				TempCelsius:      tempCelsius + rand.Intn(8) - 4,
				CollectedAt:      time.Now().Format(time.RFC3339),
			}, nil
		}
	}
	return nil, errors.New("host not found")
}

// GetHostPowerState returns the power state of a host via IPMI
func (f *FakeIPMIClient) GetHostPowerState(ctx context.Context, hostID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Simulate random power states for testing
	rand.Seed(time.Now().UnixNano() + int64(len(hostID)))
	states := []string{"on", "on", "on", "off", "suspended"}
	return states[rand.Intn(len(states))], nil
}

// SetHostPowerState sets the power state of a host via IPMI
func (f *FakeIPMIClient) SetHostPowerState(ctx context.Context, hostID string, state string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			validStates := map[string]bool{"on": true, "off": true, "suspended": true, "soft": true, "cycle": true}
			if !validStates[state] {
				return errors.New("invalid IPMI power state: " + state)
			}
			// Simulate some latency
			time.Sleep(10 * time.Millisecond)
			return nil
		}
	}
	return errors.New("host not found")
}