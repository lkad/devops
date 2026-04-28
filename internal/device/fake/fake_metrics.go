package fake

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/device"
)

// FakeMetricsCollector generates simulated metrics
type FakeMetricsCollector struct {
	BaseCPU    float64
	BaseMemory float64
	BaseDisk   float64
	BaseNet    float64

	mu sync.Mutex
}

// NewFakeMetricsCollector creates a new fake metrics collector
func NewFakeMetricsCollector() *FakeMetricsCollector {
	return &FakeMetricsCollector{
		BaseCPU:    50.0,
		BaseMemory: 60.0,
		BaseDisk:   40.0,
		BaseNet:    100.0,
	}
}

// CollectVMMetrics generates fake VM metrics
func (f *FakeMetricsCollector) CollectVMMetrics(ctx context.Context, vmID string) (*device.VMMetrics, error) {
	rand.Seed(time.Now().UnixNano() + int64(len(vmID))*1000)

	f.mu.Lock()
	cpu := f.BaseCPU + rand.Float64()*30
	mem := f.BaseMemory + rand.Float64()*25
	f.mu.Unlock()

	return &device.VMMetrics{
		VMID:       vmID,
		CPUUsage:   cpu,
		MemUsage:   mem,
		DiskIOPS:   1000 + rand.Intn(1000),
		DiskMBps:   40 + rand.Intn(100),
		NetRXMbps:  50 + rand.Float64()*500,
		NetTXMbps:  30 + rand.Float64()*400,
		CollectedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// CollectHostMetrics generates fake host metrics
func (f *FakeMetricsCollector) CollectHostMetrics(ctx context.Context, hostID string) (*device.HostMetrics, error) {
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

// CollectNetworkDeviceMetrics generates fake network device metrics
func (f *FakeMetricsCollector) CollectNetworkDeviceMetrics(ctx context.Context, deviceID string) (*device.NetworkMetrics, error) {
	rand.Seed(time.Now().UnixNano() + int64(len(deviceID))*3000)

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
		DeviceID:      deviceID,
		CPUUsage:      30 + rand.Float64()*40,
		MemoryUsage:   45 + rand.Float64()*30,
		Temperature:   35 + rand.Intn(20),
		InterfaceStats: stats,
		CollectedAt:   time.Now().Format(time.RFC3339),
	}, nil
}