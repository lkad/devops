// Package device implements the unified device-management subsystem.
// It owns the Device, DeviceGroup, and ConfigurationTemplate models
// plus the layered handler/service/repository code that exposes them
// over /api/v1/devices, /api/v1/device-groups, and
// /api/v1/configuration-templates.
//
// The package follows the project-wide layering rules:
//
//	handler -> service -> repository -> model
//
// Handler is thin: parse, call, render. Service owns validation,
// state-transition rules, and orchestration. Repository is GORM-only.
// Models are GORM structs with database.BaseModel embedded for
// UUID PK + soft-delete + timestamps.
package device

// DeviceType is the categorical kind of device. The string values
// are part of the public API contract — clients branch on them, so
// renaming a value is a breaking change.
type DeviceType string

const (
	DeviceTypePhysicalHost DeviceType = "physical_host"
	DeviceTypeContainer    DeviceType = "container"
	DeviceTypeNetwork      DeviceType = "network_device"
	DeviceTypeLoadBalancer DeviceType = "load_balancer"
	DeviceTypeCloud        DeviceType = "cloud_instance"
	DeviceTypeIoT          DeviceType = "iot_device"
)

// Valid reports whether t is one of the recognised device types.
// Used by the service layer to reject bogus create requests with
// a 400 VALIDATION_ERROR.
func (t DeviceType) Valid() bool {
	switch t {
	case DeviceTypePhysicalHost, DeviceTypeContainer, DeviceTypeNetwork,
		DeviceTypeLoadBalancer, DeviceTypeCloud, DeviceTypeIoT:
		return true
	}
	return false
}

// AllDeviceTypes returns the full set of valid device types in
// declaration order. The repository uses this to seed a filter
// default; tests use it to enumerate fixtures.
func AllDeviceTypes() []DeviceType {
	return []DeviceType{
		DeviceTypePhysicalHost, DeviceTypeContainer, DeviceTypeNetwork,
		DeviceTypeLoadBalancer, DeviceTypeCloud, DeviceTypeIoT,
	}
}

// DeviceState is the lifecycle state of a device. The four-state
// model is the one mandated by the playbook (online /
// monitoring_issue / offline / maintenance). It deliberately
// differs from the eight-state spec table — the spec table is
// aspirational; the playbook is the implementation contract.
type DeviceState string

const (
	DeviceStateOnline          DeviceState = "online"
	DeviceStateMonitoringIssue DeviceState = "monitoring_issue"
	DeviceStateOffline         DeviceState = "offline"
	DeviceStateMaintenance     DeviceState = "maintenance"
)

// Valid reports whether s is one of the recognised device states.
func (s DeviceState) Valid() bool {
	switch s {
	case DeviceStateOnline, DeviceStateMonitoringIssue,
		DeviceStateOffline, DeviceStateMaintenance:
		return true
	}
	return false
}

// Action names accepted by the /devices/:id/actions dispatcher.
// They are exported as constants so handlers, services, and tests
// all reference the same canonical strings.
const (
	ActionEnterMaintenance = "enter_maintenance"
	ActionExitMaintenance  = "exit_maintenance"
)

// ListFilter narrows the result of a Repository.List call.
// All fields are optional; the zero value returns every device.
type ListFilter struct {
	// Type filters by DeviceType (exact match).
	Type DeviceType
	// State filters by DeviceState (exact match).
	State DeviceState
	// GroupID filters by the device's group FK (exact match).
	// Empty string means "no filter".
	GroupID string
	// Search runs a case-insensitive substring match on Name.
	// Empty string means "no filter".
	Search string
	// Limit caps the page size; 0 falls back to the default
	// (20, per contracts.defaultPageSize).
	Limit int
	// Offset is the number of rows to skip; 0 means start at the
	// beginning of the result set.
	Offset int
}
