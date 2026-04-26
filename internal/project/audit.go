package project

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of audit action
type AuditAction string

const (
	ActionCreate AuditAction = "create"
	ActionUpdate AuditAction = "update"
	ActionDelete AuditAction = "delete"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         string      `json:"id"`
	Timestamp  time.Time   `json:"timestamp"`
	Username   string      `json:"username"`
	Action     AuditAction `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	EntityName string      `json:"entity_name"`
	Changes    string      `json:"changes,omitempty"`
	OldValue   string      `json:"old_value,omitempty"`
	NewValue   string      `json:"new_value,omitempty"`
	IPAddress  string      `json:"ip_address,omitempty"`
}

// NewAuditLog creates a new audit log entry
func NewAuditLog(username string, action AuditAction, entityType, entityID, entityName string) *AuditLog {
	return &AuditLog{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		Username:   username,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: entityName,
	}
}
