package fake

import (
	"context"
	"testing"

	"github.com/devops-toolkit/internal/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeVMwareClient_ListVMs(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	t.Run("list all VMs", func(t *testing.T) {
		vms, err := fake.ListVMs(ctx, "")
		require.NoError(t, err)
		assert.Len(t, vms, 3)
		assert.Equal(t, "web-server-01", vms[0].Name)
		assert.Equal(t, "db-server-01", vms[1].Name)
		assert.Equal(t, "cache-server-01", vms[2].Name)
	})

	t.Run("filter by host", func(t *testing.T) {
		vms, err := fake.ListVMs(ctx, "host-1")
		require.NoError(t, err)
		assert.Len(t, vms, 2)
		for _, vm := range vms {
			assert.Equal(t, "host-1", vm.Hypervisor)
		}
	})

	t.Run("filter by non-existent host", func(t *testing.T) {
		vms, err := fake.ListVMs(ctx, "non-existent")
		require.NoError(t, err)
		assert.Len(t, vms, 0)
	})
}

func TestFakeVMwareClient_GetVM(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	t.Run("get existing VM", func(t *testing.T) {
		vm, err := fake.GetVM(ctx, "vm-100")
		require.NoError(t, err)
		assert.Equal(t, "web-server-01", vm.Name)
		assert.Equal(t, 2, vm.VCPU)
		assert.Equal(t, 4096, vm.MemoryMB)
	})

	t.Run("get non-existent VM", func(t *testing.T) {
		_, err := fake.GetVM(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestFakeVMwareClient_GetVMMetrics(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	metrics, err := fake.GetVMMetrics(ctx, "vm-100")
	require.NoError(t, err)
	assert.Equal(t, "vm-100", metrics.VMID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
	assert.Greater(t, metrics.MemUsage, 0.0)
	assert.Less(t, metrics.MemUsage, 100.0)
	assert.Greater(t, metrics.DiskIOPS, 0)
	assert.Greater(t, metrics.NetRXMbps, 0.0)
}

func TestFakeVMwareClient_GetHostInfo(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	t.Run("get existing host", func(t *testing.T) {
		host, err := fake.GetHostInfo(ctx, "host-1")
		require.NoError(t, err)
		assert.Equal(t, "esxi-host-01", host.Name)
		assert.Equal(t, device.TypePhysicalHost, host.Type)
	})

	t.Run("get non-existent host", func(t *testing.T) {
		_, err := fake.GetHostInfo(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestFakeVMwareClient_GetHostMetrics(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	metrics, err := fake.GetHostMetrics(ctx, "host-1")
	require.NoError(t, err)
	assert.Equal(t, "host-1", metrics.HostID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
	assert.Greater(t, metrics.MemoryUsage, 0.0)
	assert.Less(t, metrics.MemoryUsage, 100.0)
	assert.Greater(t, metrics.PowerWatts, 0)
	assert.Greater(t, metrics.TempCelsius, 0)
}

func TestFakeVMwareClient_PowerState(t *testing.T) {
	fake := NewFakeVMwareClient()
	ctx := context.Background()

	t.Run("get power state", func(t *testing.T) {
		state, err := fake.GetHostPowerState(ctx, "host-1")
		require.NoError(t, err)
		assert.Equal(t, "on", state)
	})

	t.Run("set valid power state", func(t *testing.T) {
		err := fake.SetHostPowerState(ctx, "host-1", "off")
		assert.NoError(t, err)
	})

	t.Run("set invalid power state", func(t *testing.T) {
		err := fake.SetHostPowerState(ctx, "host-1", "invalid")
		assert.Error(t, err)
	})

	t.Run("set power state for non-existent host", func(t *testing.T) {
		err := fake.SetHostPowerState(ctx, "non-existent", "on")
		assert.Error(t, err)
	})
}

func TestFakeKVMClient_ListVMs(t *testing.T) {
	fake := NewFakeKVMClient()
	ctx := context.Background()

	t.Run("list all VMs", func(t *testing.T) {
		vms, err := fake.ListVMs(ctx, "")
		require.NoError(t, err)
		assert.Len(t, vms, 3)
		assert.Equal(t, "kvm-web-01", vms[0].Name)
	})

	t.Run("filter by host", func(t *testing.T) {
		vms, err := fake.ListVMs(ctx, "kvm-host-01")
		require.NoError(t, err)
		assert.Len(t, vms, 2)
		for _, vm := range vms {
			assert.Equal(t, "kvm-host-01", vm.Hypervisor)
		}
	})
}

func TestFakeKVMClient_GetVMMetrics(t *testing.T) {
	fake := NewFakeKVMClient()
	ctx := context.Background()

	metrics, err := fake.GetVMMetrics(ctx, "kvm-vm-001")
	require.NoError(t, err)
	assert.Equal(t, "kvm-vm-001", metrics.VMID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
}

func TestFakeMetricsCollector_CollectVMMetrics(t *testing.T) {
	collector := NewFakeMetricsCollector()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		metrics, err := collector.CollectVMMetrics(ctx, "vm-test")
		require.NoError(t, err)
		assert.Greater(t, metrics.CPUUsage, 0.0)
		assert.Less(t, metrics.CPUUsage, 100.0)
		assert.Greater(t, metrics.MemUsage, 0.0)
		assert.Less(t, metrics.MemUsage, 100.0)
	}
}

func TestFakeMetricsCollector_CollectHostMetrics(t *testing.T) {
	collector := NewFakeMetricsCollector()
	ctx := context.Background()

	metrics, err := collector.CollectHostMetrics(ctx, "host-test")
	require.NoError(t, err)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
	assert.Greater(t, metrics.MemoryUsage, 0.0)
	assert.Less(t, metrics.MemoryUsage, 100.0)
	assert.Greater(t, metrics.PowerWatts, 0)
	assert.Greater(t, metrics.TempCelsius, 0)
}

func TestFakeMetricsCollector_CollectNetworkDeviceMetrics(t *testing.T) {
	collector := NewFakeMetricsCollector()
	ctx := context.Background()

	metrics, err := collector.CollectNetworkDeviceMetrics(ctx, "sw-001")
	require.NoError(t, err)
	assert.Equal(t, "sw-001", metrics.DeviceID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
	assert.Greater(t, metrics.Temperature, 0)
	assert.NotEmpty(t, metrics.InterfaceStats)
}

func TestFakeNetworkDeviceClient_ListDevices(t *testing.T) {
	fake := NewFakeNetworkDeviceClient()
	ctx := context.Background()

	devices, err := fake.ListDevices(ctx)
	require.NoError(t, err)
	assert.Len(t, devices, 3)

	deviceTypes := make(map[string]bool)
	for _, d := range devices {
		deviceTypes[string(d.Type)] = true
	}
	assert.True(t, deviceTypes["network_device"])
}

func TestFakeNetworkDeviceClient_GetDevice(t *testing.T) {
	fake := NewFakeNetworkDeviceClient()
	ctx := context.Background()

	t.Run("get existing device", func(t *testing.T) {
		device, err := fake.GetDevice(ctx, "sw-001")
		require.NoError(t, err)
		assert.Equal(t, "core-switch-01", device.Name)
	})

	t.Run("get non-existent device", func(t *testing.T) {
		_, err := fake.GetDevice(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestFakeNetworkDeviceClient_GetDeviceInterfaces(t *testing.T) {
	fake := NewFakeNetworkDeviceClient()
	ctx := context.Background()

	t.Run("get Cisco switch interfaces", func(t *testing.T) {
		ifaces, err := fake.GetDeviceInterfaces(ctx, "sw-001")
		require.NoError(t, err)
		assert.Len(t, ifaces, 4)
		assert.Equal(t, "Gi0/0/1", ifaces[0].Name)
	})

	t.Run("get Juniper switch interfaces", func(t *testing.T) {
		ifaces, err := fake.GetDeviceInterfaces(ctx, "sw-002")
		require.NoError(t, err)
		assert.Len(t, ifaces, 3)
		assert.Equal(t, "ge-0/0/1", ifaces[0].Name)
	})

	t.Run("get firewall interfaces", func(t *testing.T) {
		ifaces, err := fake.GetDeviceInterfaces(ctx, "fw-001")
		require.NoError(t, err)
		assert.Len(t, ifaces, 3)
		assert.NotEmpty(t, ifaces[0].IPAddress)
	})
}

func TestFakeNetworkDeviceClient_GetDeviceMetrics(t *testing.T) {
	fake := NewFakeNetworkDeviceClient()
	ctx := context.Background()

	metrics, err := fake.GetDeviceMetrics(ctx, "sw-001")
	require.NoError(t, err)
	assert.Equal(t, "sw-001", metrics.DeviceID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
}

func TestFakeNetworkDeviceClient_BackupConfig(t *testing.T) {
	fake := NewFakeNetworkDeviceClient()
	ctx := context.Background()

	t.Run("backup existing device", func(t *testing.T) {
		result, err := fake.BackupConfig(ctx, "sw-001")
		require.NoError(t, err)
		assert.Contains(t, result, "Backup successful")
		assert.Contains(t, result, "core-switch-01")
	})

	t.Run("backup non-existent device", func(t *testing.T) {
		_, err := fake.BackupConfig(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestFakeIPMIClient_GetHostInfo(t *testing.T) {
	fake := NewFakeIPMIClient()
	ctx := context.Background()

	t.Run("get existing host", func(t *testing.T) {
		host, err := fake.GetHostInfo(ctx, "bmc-host-001")
		require.NoError(t, err)
		assert.Equal(t, "dell-r740-01", host.Name)
		assert.Equal(t, device.TypePhysicalHost, host.Type)
	})

	t.Run("get non-existent host", func(t *testing.T) {
		_, err := fake.GetHostInfo(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestFakeIPMIClient_GetHostMetrics(t *testing.T) {
	fake := NewFakeIPMIClient()
	ctx := context.Background()

	metrics, err := fake.GetHostMetrics(ctx, "bmc-host-001")
	require.NoError(t, err)
	assert.Equal(t, "bmc-host-001", metrics.HostID)
	assert.Greater(t, metrics.CPUUsage, 0.0)
	assert.Less(t, metrics.CPUUsage, 100.0)
	assert.Greater(t, metrics.PowerWatts, 0)
	assert.Greater(t, metrics.TempCelsius, 0)
}

func TestFakeIPMIClient_PowerState(t *testing.T) {
	fake := NewFakeIPMIClient()
	ctx := context.Background()

	t.Run("get power state", func(t *testing.T) {
		state, err := fake.GetHostPowerState(ctx, "bmc-host-001")
		require.NoError(t, err)
		assert.NotEmpty(t, state)
	})

	t.Run("set valid power state", func(t *testing.T) {
		err := fake.SetHostPowerState(ctx, "bmc-host-001", "off")
		assert.NoError(t, err)
	})

	t.Run("set invalid power state", func(t *testing.T) {
		err := fake.SetHostPowerState(ctx, "bmc-host-001", "invalid")
		assert.Error(t, err)
	})
}

func TestFakeIPMIClient_VMOperationsNotSupported(t *testing.T) {
	fake := NewFakeIPMIClient()
	ctx := context.Background()

	_, err := fake.ListVMs(ctx, "")
	assert.NoError(t, err)

	_, err = fake.GetVM(ctx, "vm-100")
	assert.Error(t, err)

	_, err = fake.GetVMMetrics(ctx, "vm-100")
	assert.Error(t, err)
}