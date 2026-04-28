package fake

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/device"
)

// FakeVMwareClient simulates vSphere API responses
type FakeVMwareClient struct {
	mu       sync.RWMutex
	VMs      []*device.VM
	Hosts    []*device.GORMDevice
	Metrics  map[string]*device.VMMetrics
	Latency  time.Duration
	ErrorRate float64
}

// NewFakeVMwareClient creates a new fake VMware client with default data
func NewFakeVMwareClient() *FakeVMwareClient {
	now := time.Now()
	return &FakeVMwareClient{
		VMs: []*device.VM{
			{
				ID:         "vm-100",
				Name:       "web-server-01",
				VCPU:       2,
				MemoryMB:   4096,
				State:      "running",
				Hypervisor: "host-1",
				DiskGB:     50,
				IPAddresses: []string{"192.168.1.10"},
				MACAddress: "00:0c:29:ab:cd:ef",
				GuestOS:    "ubuntu 22.04",
				Cluster:    "Production",
				CreatedAt:  now.Add(-720 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:         "vm-101",
				Name:       "db-server-01",
				VCPU:       4,
				MemoryMB:   16384,
				State:      "running",
				Hypervisor: "host-1",
				DiskGB:     200,
				IPAddresses: []string{"192.168.1.11"},
				MACAddress: "00:0c:29:ab:cd:f0",
				GuestOS:    "ubuntu 22.04",
				Cluster:    "Production",
				CreatedAt:  now.Add(-500 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:         "vm-102",
				Name:       "cache-server-01",
				VCPU:       2,
				MemoryMB:   8192,
				State:      "running",
				Hypervisor: "host-2",
				DiskGB:     100,
				IPAddresses: []string{"192.168.1.12"},
				MACAddress: "00:0c:29:ab:cd:f1",
				GuestOS:    "ubuntu 22.04",
				Cluster:    "Production",
				CreatedAt:  now.Add(-300 * time.Hour).Format(time.RFC3339),
			},
		},
		Hosts: []*device.GORMDevice{
			{
				ID:   "host-1",
				Name: "esxi-host-01",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer":     "Dell",
					"model":           "PowerEdge R740",
					"serial_no":       "SN123456",
					"cpu_cores":        32,
					"memory_gb":        128,
					"mgmt_ip":          "192.168.1.101",
					"ipmi_ip":          "192.168.1.100",
				},
			},
			{
				ID:   "host-2",
				Name: "esxi-host-02",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer":     "HP",
					"model":           "ProLiant DL380",
					"serial_no":       "SN654321",
					"cpu_cores":        64,
					"memory_gb":        256,
					"mgmt_ip":          "192.168.1.102",
					"ipmi_ip":          "192.168.1.101",
				},
			},
		},
		Metrics:  make(map[string]*device.VMMetrics),
		Latency:  50 * time.Millisecond,
		ErrorRate: 0.0,
	}
}

// ListVMs returns all VMs or VMs filtered by hostID
func (f *FakeVMwareClient) ListVMs(ctx context.Context, hostID string) ([]*device.VM, error) {
	if f.Latency > 0 {
		time.Sleep(f.Latency)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.ErrorRate > 0 && rand.Float64() < f.ErrorRate {
		return nil, errors.New("simulated vSphere API error")
	}

	if hostID == "" {
		result := make([]*device.VM, len(f.VMs))
		copy(result, f.VMs)
		return result, nil
	}

	var result []*device.VM
	for _, vm := range f.VMs {
		if vm.Hypervisor == hostID {
			result = append(result, vm)
		}
	}
	return result, nil
}

// GetVM returns a specific VM by ID
func (f *FakeVMwareClient) GetVM(ctx context.Context, vmID string) (*device.VM, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, vm := range f.VMs {
		if vm.ID == vmID {
			return vm, nil
		}
	}
	return nil, errors.New("VM not found")
}

// GetVMMetrics returns metrics for a specific VM
func (f *FakeVMwareClient) GetVMMetrics(ctx context.Context, vmID string) (*device.VMMetrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Metrics[vmID]; !ok {
		f.Metrics[vmID] = f.generateMetrics(vmID)
	}
	return f.Metrics[vmID], nil
}

// GetHostInfo returns information about a physical host
func (f *FakeVMwareClient) GetHostInfo(ctx context.Context, hostID string) (*device.GORMDevice, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			return host, nil
		}
	}
	return nil, errors.New("host not found")
}

// GetHostMetrics returns metrics for a specific host
func (f *FakeVMwareClient) GetHostMetrics(ctx context.Context, hostID string) (*device.HostMetrics, error) {
	rand.Seed(time.Now().UnixNano() + int64(len(hostID))*2000)
	return &device.HostMetrics{
		HostID:           hostID,
		CPUUsage:         30 + rand.Float64()*40,
		MemoryUsage:      50 + rand.Float64()*35,
		MemoryTotalGB:    128,
		MemoryUsedGB:     64 + rand.Intn(32),
		DiskUsagePercent: 45 + rand.Float64()*30,
		PowerWatts:       300 + rand.Intn(200),
		TempCelsius:      35 + rand.Intn(25),
		CollectedAt:      time.Now().Format(time.RFC3339),
	}, nil
}

// GetHostPowerState returns the power state of a host
func (f *FakeVMwareClient) GetHostPowerState(ctx context.Context, hostID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			return "on", nil
		}
	}
	return "", errors.New("host not found")
}

// SetHostPowerState sets the power state of a host (simulated)
func (f *FakeVMwareClient) SetHostPowerState(ctx context.Context, hostID string, state string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			if state != "on" && state != "off" && state != "suspended" {
				return errors.New("invalid power state")
			}
			return nil
		}
	}
	return errors.New("host not found")
}

func (f *FakeVMwareClient) generateMetrics(vmID string) *device.VMMetrics {
	rand.Seed(time.Now().UnixNano() + int64(len(vmID)))
	return &device.VMMetrics{
		VMID:       vmID,
		CPUUsage:   45.5 + rand.Float64()*30,
		MemUsage:   62.3 + rand.Float64()*20,
		DiskIOPS:   1200 + rand.Intn(800),
		DiskMBps:   50 + rand.Intn(100),
		NetRXMbps:  100 + rand.Float64()*400,
		NetTXMbps:  80 + rand.Float64()*300,
		CollectedAt: time.Now().Format(time.RFC3339),
	}
}