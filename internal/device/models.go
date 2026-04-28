package device

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceType constants
type DeviceType string

const (
	TypePhysicalHost  DeviceType = "physical_host"
	TypeVM             DeviceType = "vm"
	TypeContainer      DeviceType = "container"
	TypeNetworkDevice  DeviceType = "network_device"
	TypeLoadBalancer   DeviceType = "load_balancer"
	TypeCloudInstance  DeviceType = "cloud_instance"
	TypeIoTDevice      DeviceType = "iot_device"
)

// State constants for all device types
type State string

const (
	// Common/Generic states
	StatePending     State = "pending"
	StateActive      State = "active"
	StateInactive    State = "inactive"
	StateMaintenance State = "maintenance"
	StateRetired     State = "retired"
	StateFailed      State = "failed"

	// Physical host states
	StateAuthenticated State = "authenticated"
	StateRegistered     State = "registered"
	StateSuspended      State = "suspended"

	// VM states
	StateRunning    State = "running"
	StateStopped   State = "stopped"
	StateTerminated State = "terminated"

	// Network device states
	StateDiscovered State = "discovered"
)

// JSONMap and StringMap types
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

type StringMap map[string]string

func (s StringMap) Value() (driver.Value, error) {
	if s == nil {
		return "{}", nil
	}
	return json.Marshal(s)
}

func (s *StringMap) Scan(value interface{}) error {
	if s == nil {
		*s = make(StringMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, s)
}

// PhysicalHostConfig holds physical host specific configuration
type PhysicalHostConfig struct {
	Manufacturer        string   `json:"manufacturer"`
	Model              string   `json:"model"`
	SerialNo           string   `json:"serial_no"`
	BIOSVersion        string   `json:"bios_version"`
	HardwareUUID       string   `json:"hardware_uuid"`
	CPUModel           string   `json:"cpu_model"`
	CPUCores           int      `json:"cpu_cores"`
	CPUThreads         int      `json:"cpu_threads"`
	MemoryGB           int      `json:"memory_gb"`
	MemorySlotsUsed    int      `json:"memory_slots_used"`
	MemorySlotsTotal   int      `json:"memory_slots_total"`
	DiskTotalGB        int      `json:"disk_total_gb"`
	DiskConfig         JSONMap  `json:"disk_config"`
	MgmtIP             string   `json:"mgmt_ip"`
	IPMIIP             string   `json:"ipmi_ip"`
	MACAddresses       []string `json:"mac_addresses"`
	Location           string   `json:"location"`
	Rack               string   `json:"rack"`
	RackPosition       int      `json:"rack_position"`
	RackUnits          int      `json:"rack_units"`
	PowerSupplyCount   int      `json:"power_supply_count"`
	PowerConsumptionW  int      `json:"power_consumption_watts"`
	PDUInfo            JSONMap  `json:"pdu_info"`
	RedundantPSU       bool     `json:"redundant_psu"`
	AssetNo            string   `json:"asset_no"`
	PurchaseDate       string   `json:"purchase_date"`
	WarrantyExpire     string   `json:"warranty_expire"`
	OwnerTeam          string   `json:"owner_team"`
}

// VMConfig holds VM specific configuration
type VMConfig struct {
	VMID                string   `json:"vm_id"`
	HypervisorType      string   `json:"hypervisor_type"`
	HypervisorHost      string   `json:"hypervisor_host"`
	ResourcePool        string   `json:"resource_pool"`
	Cluster             string   `json:"cluster"`
	VCPU                int      `json:"vcpu"`
	VCPUReservation     int      `json:"vcpu_reservation"`
	MemoryMB            int      `json:"memory_mb"`
	MemoryReservationMB int      `json:"memory_reservation_mb"`
	MemoryLimitMB       int      `json:"memory_limit_mb"`
	DiskTotalGB         int      `json:"disk_total_gb"`
	DiskSnapshotCount   int      `json:"disk_snapshot_count"`
	DiskDatastore       string   `json:"disk_datastore"`
	DiskPath            string   `json:"disk_path"`
	Interfaces          JSONMap  `json:"interfaces"`
	IPAddresses         []string `json:"ip_addresses"`
	MACAddress          string   `json:"mac_address"`
	PortGroup           string   `json:"port_group"`
	PowerState          string   `json:"power_state"`
	GuestOS             string   `json:"guest_os"`
	ToolsStatus         string   `json:"tools_status"`
	CreatedTime         string   `json:"created_time"`
	Template            string   `json:"template"`
	TemplateName        string   `json:"template_name"`
}

// NetworkDeviceConfig holds network device specific configuration
type NetworkDeviceConfig struct {
	DeviceType          string   `json:"device_type"`
	Vendor              string   `json:"vendor"`
	Model               string   `json:"model"`
	OSVersion           string   `json:"os_version"`
	SerialNo            string   `json:"serial_no"`
	FirmwareVersion    string   `json:"firmware_version"`
	MgmtIP              string   `json:"mgmt_ip"`
	MgmtVRF             string   `json:"mgmt_vrf"`
	ConsoleIP           string   `json:"console_ip"`
	ConsolePort         string   `json:"console_port"`
	Interfaces          JSONMap  `json:"interfaces"`
	PortChannels        JSONMap  `json:"port_channels"`
	VLANS               JSONMap  `json:"vlans"`
	SpanningTree        JSONMap  `json:"spanning_tree"`
	RoutingInstances    JSONMap  `json:"routing_instances"`
	BGPConfig           JSONMap  `json:"bgp_config"`
	OSPFConfig          JSONMap  `json:"ospf_config"`
	StaticRoutes        JSONMap  `json:"static_routes"`
	ACLConfig           JSONMap  `json:"acl_config"`
	PortSecurity        JSONMap  `json:"port_security"`
	Dot1xConfig         JSONMap  `json:"dot1x_config"`
	ConfigBackupLast    string   `json:"config_backup_last"`
	ConfigBackupStatus  string   `json:"config_backup_status"`
	ConfigDiffFromBase  bool     `json:"config_diff_from_baseline"`
	SNMPReadCommunity   string   `json:"snmp_read_community"`
	SNMPWriteCommunity   string   `json:"snmp_write_community"`
	SNMPv3Config        JSONMap  `json:"snmp_v3_config"`
	SNMPTrapTargets     []string `json:"snmp_trap_targets"`
}

// NetworkInterface represents a network device interface
type NetworkInterface struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AdminStatus string `json:"admin_status"`
	OperStatus  string `json:"oper_status"`
	Speed       string `json:"speed"`
	Duplex      string `json:"duplex"`
	MTU         int    `json:"mtu"`
	VLANAccess  int    `json:"vlan_access"`
	VLANTrunk   []int  `json:"vlan_trunk"`
	Layer       int    `json:"layer"`
	IPAddress   string `json:"ip_address"`
	PVID        int    `json:"pvid"`
	POEStatus   string `json:"poe_status"`
	MACAddress  string `json:"mac_address"`
}

// InterfaceStats represents interface statistics
type InterfaceStats struct {
	InBytes   int64 `json:"in_bytes"`
	OutBytes  int64 `json:"out_bytes"`
	InErrors  int   `json:"in_errors"`
	OutErrors int   `json:"out_errors"`
}

// DeviceStateTransition records state changes
type DeviceStateTransition struct {
	gorm.Model
	DeviceID    uuid.UUID `gorm:"type:uuid;not null" json:"device_id"`
	FromState   string    `gorm:"type:text" json:"from_state"`
	ToState     string    `gorm:"type:text;not null" json:"to_state"`
	TriggeredBy string    `gorm:"type:text" json:"triggered_by"`
	Reason      string    `gorm:"type:text" json:"reason,omitempty"`
}

// DeviceGroup for grouping devices
type DeviceGroup struct {
	gorm.Model
	Name     string     `gorm:"type:varchar(255);not null" json:"name"`
	ParentID *uuid.UUID `gorm:"type:uuid" json:"parent_id,omitempty"`
	Type     string     `gorm:"type:varchar(50)" json:"type"`
	Criteria JSONMap    `gorm:"type:jsonb" json:"criteria"`
}

// GORMDevice is the GORM model for Device
type GORMDevice struct {
	gorm.Model
	ID             string     `gorm:"type:text;primaryKey" json:"id"`
	Type           DeviceType `gorm:"type:text;not null" json:"type"`
	Name           string     `gorm:"type:varchar(255);not null" json:"name"`
	Status         string     `gorm:"type:text;not null;default:'pending'" json:"status"`
	Environment    string     `gorm:"type:text;not null;default:'dev'" json:"environment"`
	Labels         StringMap  `gorm:"type:jsonb;serializer:json" json:"labels"`
	BusinessUnit   string     `gorm:"type:text" json:"business_unit,omitempty"`
	ComputeCluster string     `gorm:"type:text" json:"compute_cluster,omitempty"`
	ParentID       string     `gorm:"type:text" json:"parent_id,omitempty"`
	Config         JSONMap    `gorm:"type:jsonb;serializer:json" json:"config,omitempty"`
	Metadata       JSONMap    `gorm:"type:jsonb;serializer:json" json:"metadata,omitempty"`
	RegisteredAt   *time.Time `gorm:"type:timestamp" json:"registered_at,omitempty"`
	LastSeen       *time.Time `gorm:"type:timestamp" json:"last_seen,omitempty"`
	LastConfigSync *time.Time `gorm:"type:timestamp" json:"last_config_sync,omitempty"`
}

func (GORMDevice) TableName() string {
	return "devices"
}

// validTransitions defines allowed state transitions per device type
var validTransitions = map[State][]State{
	// Physical host transitions
	StatePending:      {StateAuthenticated, StateFailed},
	StateAuthenticated: {StateRegistered, StateFailed},
	StateRegistered:    {StateActive, StateRetired},
	StateActive:        {StateMaintenance, StateSuspended},
	StateMaintenance:   {StateActive},
	StateSuspended:     {StateActive, StateRetired},
	StateRetired:       {},

	// VM transitions
	StateRunning:    {StateStopped, StateSuspended},
	StateStopped:    {StateRunning, StateTerminated},
	StateTerminated: {},

	// Network device transitions
	StateDiscovered: {StateActive},
	StateFailed:    {},
}

// CanTransitionTo checks if transition to new state is valid
func (s State) CanTransitionTo(newState State) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == newState {
			return true
		}
	}
	return false
}

// IsTerminal checks if state is terminal (no further transitions)
func (s State) IsTerminal() bool {
	return s == StateRetired || s == StateTerminated
}