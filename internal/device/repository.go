package device

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) migrate() error {
	return nil // GORM AutoMigrate handles this
}

func (r *Repository) Create(d *Device) error {
	device := r.toGORMDevice(d)
	return r.db.Create(device).Error
}

func (r *Repository) GetByID(id string) (*Device, error) {
	var device GORMDevice
	if err := r.db.First(&device, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.fromGORMDevice(&device), nil
}

func (r *Repository) List() ([]*Device, error) {
	var devices []GORMDevice
	if err := r.db.Order("created_at DESC").Find(&devices).Error; err != nil {
		return nil, err
	}
	return r.fromGORMDevices(devices), nil
}

func (r *Repository) ListPaginated(limit, offset int) ([]*Device, int, error) {
	var total int64
	if err := r.db.Model(&GORMDevice{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []GORMDevice
	if err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&devices).Error; err != nil {
		return nil, 0, err
	}
	return r.fromGORMDevices(devices), int(total), nil
}

// ListByType returns devices filtered by type
func (r *Repository) ListByType(deviceType string, limit, offset int) ([]*Device, int, error) {
	var total int64
	if err := r.db.Model(&GORMDevice{}).Where("type = ?", deviceType).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []GORMDevice
	if err := r.db.Where("type = ?", deviceType).Order("created_at DESC").Limit(limit).Offset(offset).Find(&devices).Error; err != nil {
		return nil, 0, err
	}
	return r.fromGORMDevices(devices), int(total), nil
}

func (r *Repository) Update(d *Device) error {
	d.UpdatedAt = time.Now()
	device := r.toGORMDevice(d)
	return r.db.Save(device).Error
}

func (r *Repository) Delete(id string) error {
	return r.db.Delete(&GORMDevice{}, "id = ?", id).Error
}

func (r *Repository) UpdateStatus(id string, status State) error {
	return r.db.Model(&GORMDevice{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

// SearchByLabels searches devices by label key-value pairs
func (r *Repository) SearchByLabels(labels map[string]string) ([]*Device, error) {
	query := r.db.Model(&GORMDevice{})
	for k, v := range labels {
		query = query.Where("labels->>? = ?", k, v)
	}
	var devices []GORMDevice
	if err := query.Find(&devices).Error; err != nil {
		return nil, err
	}
	return r.fromGORMDevices(devices), nil
}

// RecordStateTransition logs a state change
func (r *Repository) RecordStateTransition(deviceID string, fromState, toState State, triggeredBy, reason string) error {
	uid, err := uuid.Parse(deviceID)
	if err != nil {
		return err
	}
	transition := &DeviceStateTransition{
		DeviceID:    uid,
		FromState:   string(fromState),
		ToState:     string(toState),
		TriggeredBy: triggeredBy,
		Reason:      reason,
	}
	return r.db.Create(transition).Error
}

// GetStateHistory retrieves state transition history for a device
func (r *Repository) GetStateHistory(deviceID string) ([]*DeviceStateTransition, error) {
	uid, err := uuid.Parse(deviceID)
	if err != nil {
		return nil, err
	}
	var transitions []DeviceStateTransition
	if err := r.db.Where("device_id = ?", uid).Order("created_at DESC").Find(&transitions).Error; err != nil {
		return nil, err
	}
	result := make([]*DeviceStateTransition, len(transitions))
	for i := range transitions {
		result[i] = &transitions[i]
	}
	return result, nil
}

// ListByStatus returns devices filtered by status
func (r *Repository) ListByStatus(status string, limit, offset int) ([]*Device, int, error) {
	var total int64
	if err := r.db.Model(&GORMDevice{}).Where("status = ?", status).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []GORMDevice
	if err := r.db.Where("status = ?", status).Order("created_at DESC").Limit(limit).Offset(offset).Find(&devices).Error; err != nil {
		return nil, 0, err
	}
	return r.fromGORMDevices(devices), int(total), nil
}

// ListByParent returns child devices of a parent
func (r *Repository) ListByParent(parentID string) ([]*Device, error) {
	var devices []GORMDevice
	if err := r.db.Where("parent_id = ?", parentID).Find(&devices).Error; err != nil {
		return nil, err
	}
	return r.fromGORMDevices(devices), nil
}

func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// toGORMDevice converts Device to GORMDevice
func (r *Repository) toGORMDevice(d *Device) *GORMDevice {
	labels := StringMap{}
	if d.Labels != nil {
		labels = d.Labels
	}
	return &GORMDevice{
		ID:             d.ID,
		Type:           DeviceType(d.Type),
		Name:           d.Name,
		Status:         string(d.Status),
		Environment:    string(d.Environment),
		Labels:         labels,
		BusinessUnit:   d.BusinessUnit,
		ComputeCluster: d.ComputeCluster,
		ParentID:       d.ParentID,
		Config:         d.Config,
		Metadata:       d.Metadata,
		RegisteredAt:   d.RegisteredAt,
		LastSeen:       d.LastSeen,
		LastConfigSync: d.LastConfigSync,
		Model: gorm.Model{
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		},
	}
}

// fromGORMDevice converts GORMDevice to Device
func (r *Repository) fromGORMDevice(d *GORMDevice) *Device {
	labels := map[string]string{}
	if d.Labels != nil {
		labels = d.Labels
	}
	return &Device{
		ID:             d.ID,
		Type:           string(d.Type),
		Name:           d.Name,
		Status:         State(d.Status),
		Environment:    Environment(d.Environment),
		Labels:         labels,
		BusinessUnit:   d.BusinessUnit,
		ComputeCluster: d.ComputeCluster,
		ParentID:       d.ParentID,
		Config:         d.Config,
		Metadata:       d.Metadata,
		RegisteredAt:   d.RegisteredAt,
		LastSeen:       d.LastSeen,
		LastConfigSync: d.LastConfigSync,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

// fromGORMDevices converts multiple GORMDevices to Devices
func (r *Repository) fromGORMDevices(devices []GORMDevice) []*Device {
	result := make([]*Device, len(devices))
	for i := range devices {
		result[i] = r.fromGORMDevice(&devices[i])
	}
	return result
}