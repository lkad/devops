package fake

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/device"
)

// FakeKVMClient simulates KVM/libvirt API responses
type FakeKVMClient struct {
	mu       sync.RWMutex
	VMs      []*device.VM
	Hosts    []*device.GORMDevice
	Metrics  map[string]*device.VMMetrics
	Latency  time.Duration
}

// NewFakeKVMClient creates a new fake KVM client
func NewFakeKVMClient() *FakeKVMClient {
	now := time.Now()
	return &FakeKVMClient{
		VMs: []*device.VM{
			{
				ID:         "kvm-vm-001",
				Name:       "kvm-web-01",
				VCPU:       4,
				MemoryMB:   8192,
				State:      "running",
				Hypervisor: "kvm-host-01",
				DiskGB:     80,
				IPAddresses: []string{"192.168.2.10"},
				MACAddress: "52:54:00:ab:cd:ef",
				GuestOS:    "centos 8",
				Cluster:    "KVM-Cluster",
				CreatedAt:  now.Add(-200 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:         "kvm-vm-002",
				Name:       "kvm-db-01",
				VCPU:       8,
				MemoryMB:   32768,
				State:      "running",
				Hypervisor: "kvm-host-01",
				DiskGB:     500,
				IPAddresses: []string{"192.168.2.11"},
				MACAddress: "52:54:00:ab:cd:f0",
				GuestOS:    "centos 8",
				Cluster:    "KVM-Cluster",
				CreatedAt:  now.Add(-100 * time.Hour).Format(time.RFC3339),
			},
			{
				ID:         "kvm-vm-003",
				Name:       "kvm-app-01",
				VCPU:       2,
				MemoryMB:   4096,
				State:      "running",
				Hypervisor: "kvm-host-02",
				DiskGB:     100,
				IPAddresses: []string{"192.168.2.12"},
				MACAddress: "52:54:00:ab:cd:f1",
				GuestOS:    "ubuntu 20.04",
				Cluster:    "KVM-Cluster",
				CreatedAt:  now.Add(-50 * time.Hour).Format(time.RFC3339),
			},
		},
		Hosts: []*device.GORMDevice{
			{
				ID:   "kvm-host-01",
				Name: "kvm-host-01",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer": "Supermicro",
					"model":        "X11DPI-N",
					"serial_no":    "SKVM123456",
					"cpu_cores":    48,
					"memory_gb":    256,
					"mgmt_ip":      "192.168.2.101",
				},
			},
			{
				ID:   "kvm-host-02",
				Name: "kvm-host-02",
				Type: device.TypePhysicalHost,
				Config: device.JSONMap{
					"manufacturer": "Supermicro",
					"model":        "X11DPI-N",
					"serial_no":    "SKVM654321",
					"cpu_cores":    48,
					"memory_gb":    512,
					"mgmt_ip":      "192.168.2.102",
				},
			},
		},
		Metrics: make(map[string]*device.VMMetrics),
		Latency: 20 * time.Millisecond,
	}
}

// ListVMs returns all KVM VMs or VMs filtered by hostID
func (f *FakeKVMClient) ListVMs(ctx context.Context, hostID string) ([]*device.VM, error) {
	if f.Latency > 0 {
		time.Sleep(f.Latency)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

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
func (f *FakeKVMClient) GetVM(ctx context.Context, vmID string) (*device.VM, error) {
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
func (f *FakeKVMClient) GetVMMetrics(ctx context.Context, vmID string) (*device.VMMetrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Metrics[vmID]; !ok {
		f.Metrics[vmID] = f.generateMetrics(vmID)
	}
	return f.Metrics[vmID], nil
}

// GetHostInfo returns information about a physical host
func (f *FakeKVMClient) GetHostInfo(ctx context.Context, hostID string) (*device.GORMDevice, error) {
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
func (f *FakeKVMClient) GetHostMetrics(ctx context.Context, hostID string) (*device.HostMetrics, error) {
	rand.Seed(time.Now().UnixNano() + int64(len(hostID))*2000)
	return &device.HostMetrics{
		HostID:           hostID,
		CPUUsage:         25 + rand.Float64()*35,
		MemoryUsage:      55 + rand.Float64()*30,
		MemoryTotalGB:    256,
		MemoryUsedGB:     128 + rand.Intn(64),
		DiskUsagePercent: 40 + rand.Float64()*25,
		PowerWatts:       250 + rand.Intn(150),
		TempCelsius:      38 + rand.Intn(15),
		CollectedAt:      time.Now().Format(time.RFC3339),
	}, nil
}

// GetHostPowerState returns the power state of a host
func (f *FakeKVMClient) GetHostPowerState(ctx context.Context, hostID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, host := range f.Hosts {
		if host.ID == hostID {
			return "on", nil
		}
	}
	return "", errors.New("host not found")
}

// SetHostPowerState sets the power state of a host
func (f *FakeKVMClient) SetHostPowerState(ctx context.Context, hostID string, state string) error {
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

func (f *FakeKVMClient) generateMetrics(vmID string) *device.VMMetrics {
	rand.Seed(time.Now().UnixNano() + int64(len(vmID))*7) // Different seed than VMware
	return &device.VMMetrics{
		VMID:       vmID,
		CPUUsage:   40 + rand.Float64()*35,
		MemUsage:   55 + rand.Float64()*25,
		DiskIOPS:   800 + rand.Intn(600),
		DiskMBps:   30 + rand.Intn(80),
		NetRXMbps:  40 + rand.Float64()*300,
		NetTXMbps:  25 + rand.Float64()*250,
		CollectedAt: time.Now().Format(time.RFC3339),
	}
}