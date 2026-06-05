package device

import (
	"testing"
)

// TestDeviceType_Valid covers the type-validation helper. It exists
// because the service layer depends on Valid() to reject bogus
// types with a 400 VALIDATION_ERROR; the contract is "every
// declared constant is valid, the empty string and arbitrary
// strings are not".
func TestDeviceType_Valid(t *testing.T) {
	tests := []struct {
		name string
		in   DeviceType
		want bool
	}{
		{"physical_host is valid", DeviceTypePhysicalHost, true},
		{"container is valid", DeviceTypeContainer, true},
		{"network_device is valid", DeviceTypeNetwork, true},
		{"load_balancer is valid", DeviceTypeLoadBalancer, true},
		{"cloud_instance is valid", DeviceTypeCloud, true},
		{"iot_device is valid", DeviceTypeIoT, true},
		{"empty string is invalid", DeviceType(""), false},
		{"unknown string is invalid", DeviceType("satellite"), false},
		{"uppercase is invalid (case sensitive)", DeviceType("PHYSICAL_HOST"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Valid(); got != tt.want {
				t.Errorf("DeviceType(%q).Valid() = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestAllDeviceTypes_CoversAllConstants guards against the common
// mistake of declaring a new DeviceType constant but forgetting to
// list it in AllDeviceTypes. The order is part of the public
// contract (used by the filter UI to seed a dropdown).
func TestAllDeviceTypes_CoversAllConstants(t *testing.T) {
	got := AllDeviceTypes()
	want := []DeviceType{
		DeviceTypePhysicalHost, DeviceTypeContainer, DeviceTypeNetwork,
		DeviceTypeLoadBalancer, DeviceTypeCloud, DeviceTypeIoT,
	}
	if len(got) != len(want) {
		t.Fatalf("AllDeviceTypes len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllDeviceTypes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDeviceState_Valid covers the state-validation helper. The
// playbook mandates exactly four states; this test pins them.
func TestDeviceState_Valid(t *testing.T) {
	tests := []struct {
		name string
		in   DeviceState
		want bool
	}{
		{"online is valid", DeviceStateOnline, true},
		{"monitoring_issue is valid", DeviceStateMonitoringIssue, true},
		{"offline is valid", DeviceStateOffline, true},
		{"maintenance is valid", DeviceStateMaintenance, true},
		{"empty is invalid", DeviceState(""), false},
		{"unknown is invalid", DeviceState("retired"), false},
		{"uppercase is invalid", DeviceState("ONLINE"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Valid(); got != tt.want {
				t.Errorf("DeviceState(%q).Valid() = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestListFilter_ZeroValueReturnsAll pins the "zero value is no
// filter" contract. The repository depends on this so the default
// "list every device" call needs no constructor.
func TestListFilter_ZeroValueReturnsAll(t *testing.T) {
	var f ListFilter
	if f.Type != "" {
		t.Errorf("Type zero value = %q, want \"\"", f.Type)
	}
	if f.State != "" {
		t.Errorf("State zero value = %q, want \"\"", f.State)
	}
	if f.GroupID != "" {
		t.Errorf("GroupID zero value = %q, want \"\"", f.GroupID)
	}
	if f.Search != "" {
		t.Errorf("Search zero value = %q, want \"\"", f.Search)
	}
	if f.Limit != 0 {
		t.Errorf("Limit zero value = %d, want 0", f.Limit)
	}
	if f.Offset != 0 {
		t.Errorf("Offset zero value = %d, want 0", f.Offset)
	}
}
