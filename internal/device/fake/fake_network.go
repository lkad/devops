package fake

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/device"
)

// FakeNetworkDeviceClient simulates SNMP-based network device responses
type FakeNetworkDeviceClient struct {
	mu      sync.RWMutex
	Devices []*device.GORMDevice
	Latency time.Duration
}

// NewFakeNetworkDeviceClient creates a new fake network device client
func NewFakeNetworkDeviceClient() *FakeNetworkDeviceClient {
	return &FakeNetworkDeviceClient{
		Devices: []*device.GORMDevice{
			{
				ID:   "sw-001",
				Name: "core-switch-01",
				Type: device.TypeNetworkDevice,
				Config: device.JSONMap{
					"device_type":   "switch",
					"vendor":        "Cisco",
					"model":         "Catalyst 9300",
					"os_version":    "17.6.1",
					"serial_no":     "FCW2233L0AA",
					"mgmt_ip":       "192.168.1.1",
					"state":         "active",
				},
			},
			{
				ID:   "sw-002",
				Name: "access-switch-01",
				Type: device.TypeNetworkDevice,
				Config: device.JSONMap{
					"device_type":   "switch",
					"vendor":        "Juniper",
					"model":         "EX4300",
					"os_version":    "12.3R12.3",
					"serial_no":     "JN123456789",
					"mgmt_ip":       "192.168.1.2",
					"state":         "active",
				},
			},
			{
				ID:   "fw-001",
				Name: "edge-firewall-01",
				Type: device.TypeNetworkDevice,
				Config: device.JSONMap{
					"device_type":   "firewall",
					"vendor":        "Huawei",
					"model":         "USG6555E",
					"os_version":    "5.0.1",
					"serial_no":     "HW12345678",
					"mgmt_ip":       "192.168.1.3",
					"state":         "active",
				},
			},
		},
		Latency: 30 * time.Millisecond,
	}
}

// ListDevices returns all network devices
func (f *FakeNetworkDeviceClient) ListDevices(ctx context.Context) ([]*device.GORMDevice, error) {
	if f.Latency > 0 {
		time.Sleep(f.Latency)
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*device.GORMDevice, len(f.Devices))
	copy(result, f.Devices)
	return result, nil
}

// GetDevice returns a specific network device
func (f *FakeNetworkDeviceClient) GetDevice(ctx context.Context, deviceID string) (*device.GORMDevice, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, d := range f.Devices {
		if d.ID == deviceID {
			return d, nil
		}
	}
	return nil, errors.New("network device not found")
}

// GetDeviceInterfaces returns the interfaces for a network device
func (f *FakeNetworkDeviceClient) GetDeviceInterfaces(ctx context.Context, deviceID string) ([]*device.NetworkInterface, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check if device exists
	found := false
	for _, d := range f.Devices {
		if d.ID == deviceID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("network device not found")
	}

	// Return mock interfaces based on device type
	switch deviceID {
	case "sw-001":
		return []*device.NetworkInterface{
			{Name: "Gi0/0/1", Description: "To-Core-SW-02", Status: "up", Speed: "10G", VLAN: 100, MACAddress: "aabb.ccdd.ee01"},
			{Name: "Gi0/0/2", Description: "To-Access-SW-01", Status: "up", Speed: "10G", VLAN: 200, MACAddress: "aabb.ccdd.ee02"},
			{Name: "Gi0/0/3", Description: "To-FW-01", Status: "up", Speed: "10G", VLAN: 300, MACAddress: "aabb.ccdd.ee03"},
			{Name: "Gi0/0/4", Description: "Unused", Status: "down", Speed: "1G", VLAN: 1, MACAddress: "aabb.ccdd.ee04"},
		}, nil
	case "sw-002":
		return []*device.NetworkInterface{
			{Name: "ge-0/0/1", Description: "To-Core", Status: "up", Speed: "10G", VLAN: 100},
			{Name: "ge-0/0/2", Description: "To-Server", Status: "up", Speed: "1G", VLAN: 200},
			{Name: "ge-0/0/3", Description: "Unused", Status: "down", Speed: "1G", VLAN: 1},
		}, nil
	case "fw-001":
		return []*device.NetworkInterface{
			{Name: "GigabitEthernet0/0/0", Description: "WAN", Status: "up", Speed: "1G", VLAN: 0, IPAddress: "10.0.0.1/24"},
			{Name: "GigabitEthernet0/0/1", Description: "LAN", Status: "up", Speed: "10G", VLAN: 100, IPAddress: "192.168.1.1/24"},
			{Name: "GigabitEthernet0/0/2", Description: "DMZ", Status: "up", Speed: "10G", VLAN: 200, IPAddress: "172.16.0.1/24"},
		}, nil
	default:
		return []*device.NetworkInterface{}, nil
	}
}

// GetDeviceMetrics returns metrics for a network device
func (f *FakeNetworkDeviceClient) GetDeviceMetrics(ctx context.Context, deviceID string) (*device.NetworkMetrics, error) {
	rand.Seed(time.Now().UnixNano())

	stats := make(map[string]*device.InterfaceStats)
	for i := 1; i <= 4; i++ {
		ifaceName := "Gi0/0/" + string(rune('0'+i))
		stats[ifaceName] = &device.InterfaceStats{
			InBytes:   int64(1000000 + rand.Intn(9000000)),
			OutBytes:  int64(800000 + rand.Intn(7000000)),
			InErrors:  rand.Intn(5),
			OutErrors: rand.Intn(5),
		}
	}

	return &device.NetworkMetrics{
		DeviceID:       deviceID,
		CPUUsage:       30 + rand.Float64()*40,
		MemoryUsage:    45 + rand.Float64()*30,
		Temperature:    35 + rand.Intn(20),
		InterfaceStats: stats,
		CollectedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

// BackupConfig simulates config backup
func (f *FakeNetworkDeviceClient) BackupConfig(ctx context.Context, deviceID string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, d := range f.Devices {
		if d.ID == deviceID {
			return time.Now().Format(time.RFC3339) + " - Backup successful for " + d.Name, nil
		}
	}
	return "", errors.New("network device not found")
}