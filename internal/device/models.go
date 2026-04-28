package device

import (
	"time"

	"github.com/google/uuid"
)

type DeviceType string

const (
	TypePhysicalHost   DeviceType = "physical_host"
	TypeContainer     DeviceType = "container"
	TypeNetworkDevice DeviceType = "network_device"
	TypeLoadBalancer  DeviceType = "load_balancer"
	TypeCloudInstance DeviceType = "cloud_instance"
	TypeIoTDevice    DeviceType = "iot_device"
)

type JSONMap map[string]interface{}

type DeviceStateTransition struct {
	ID           uuid.UUID `json:"id"`
	DeviceID     uuid.UUID `json:"deviceId"`
	FromState    string    `json:"fromState"`
	ToState      string    `json:"toState"`
	TriggeredBy  string    `json:"triggeredBy"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type DeviceGroup struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	ParentID  *uuid.UUID `json:"parentId,omitempty"`
	Type      string     `json:"type"` // flat, hierarchical, dynamic
	Criteria  JSONMap   `json:"criteria"`
	CreatedAt time.Time  `json:"createdAt"`
}

