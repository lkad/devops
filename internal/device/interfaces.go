package device

import (
	"context"
)

// VM represents a virtual machine
type VM struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	VCPU          int      `json:"vcpu"`
	MemoryMB      int      `json:"memory_mb"`
	State         string   `json:"state"`
	Hypervisor    string   `json:"hypervisor"`
	DiskGB        int      `json:"disk_gb"`
	IPAddresses   []string `json:"ip_addresses"`
	MACAddress    string   `json:"mac_address"`
	GuestOS       string   `json:"guest_os"`
	Cluster       string   `json:"cluster"`
	ResourcePool  string   `json:"resource_pool"`
	CreatedAt     string   `json:"created_at"`
}

// VMMetrics represents VM monitoring metrics
type VMMetrics struct {
	VMID       string  `json:"vm_id"`
	CPUUsage   float64 `json:"cpu_usage"`
	MemUsage   float64 `json:"mem_usage"`
	DiskIOPS   int     `json:"disk_iops"`
	DiskMBps   int     `json:"disk_mbps"`
	NetRXMbps  float64 `json:"net_rx_mbps"`
	NetTXMbps  float64 `json:"net_tx_mbps"`
	CollectedAt string  `json:"collected_at"`
}

// HostMetrics represents physical host monitoring metrics
type HostMetrics struct {
	HostID            string  `json:"host_id"`
	CPUUsage          float64 `json:"cpu_usage"`
	MemoryUsage       float64 `json:"memory_usage"`
	MemoryTotalGB     int     `json:"memory_total_gb"`
	MemoryUsedGB      int     `json:"memory_used_gb"`
	DiskUsagePercent  float64 `json:"disk_usage_percent"`
	PowerWatts        int     `json:"power_watts"`
	TempCelsius       int     `json:"temp_celsius"`
	CollectedAt       string  `json:"collected_at"`
}

// NetworkMetrics represents network device monitoring metrics
type NetworkMetrics struct {
	DeviceID       string  `json:"device_id"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	Temperature    int     `json:"temperature"`
	InterfaceStats map[string]*InterfaceStats `json:"interface_stats"`
	CollectedAt    string  `json:"collected_at"`
}

// InterfaceStats represents network interface statistics
type InterfaceStats struct {
	InBytes    int64 `json:"in_bytes"`
	OutBytes   int64 `json:"out_bytes"`
	InErrors   int   `json:"in_errors"`
	OutErrors  int   `json:"out_errors"`
}

// HypervisorClient is the interface for hypervisor operations
type HypervisorClient interface {
	// VM Operations
	ListVMs(ctx context.Context, hostID string) ([]*VM, error)
	GetVM(ctx context.Context, vmID string) (*VM, error)
	GetVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)

	// Host Operations
	GetHostInfo(ctx context.Context, hostID string) (*GORMDevice, error)
	GetHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)

	// Power Operations
	GetHostPowerState(ctx context.Context, hostID string) (string, error)
	SetHostPowerState(ctx context.Context, hostID string, state string) error
}

// MetricsCollector is the interface for metrics collection
type MetricsCollector interface {
	CollectVMMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
	CollectHostMetrics(ctx context.Context, hostID string) (*HostMetrics, error)
	CollectNetworkDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
}

// NetworkDeviceClient is the interface for network device operations
type NetworkDeviceClient interface {
	ListDevices(ctx context.Context) ([]*GORMDevice, error)
	GetDevice(ctx context.Context, deviceID string) (*GORMDevice, error)
	GetDeviceInterfaces(ctx context.Context, deviceID string) ([]*NetworkInterface, error)
	GetDeviceMetrics(ctx context.Context, deviceID string) (*NetworkMetrics, error)
	BackupConfig(ctx context.Context, deviceID string) (string, error)
}

// NetworkInterface represents a network device interface
type NetworkInterface struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Speed       string `json:"speed"`
	VLAN        int    `json:"vlan,omitempty"`
	MACAddress  string `json:"mac_address,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	MTU         int    `json:"mtu,omitempty"`
	 Duplex     string `json:"duplex,omitempty"`
	POEStatus   string `json:"poe_status,omitempty"`
}