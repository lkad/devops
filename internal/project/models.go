package project

import (
	"time"

	"github.com/google/uuid"
)

// ProjectType represents the type of project
type ProjectType string

const (
	ProjectTypeFrontend ProjectType = "frontend"
	ProjectTypeBackend  ProjectType = "backend"
)

// ResourceType represents types of linkable resources
type ResourceType string

const (
	ResourceTypeDevice        ResourceType = "device"
	ResourceTypePipeline     ResourceType = "pipeline"
	ResourceTypeLogSource    ResourceType = "log_source"
	ResourceTypeAlertChannel ResourceType = "alert_channel"
	ResourceTypePhysicalHost ResourceType = "physical_host"
)

// Role represents permission roles
type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

// BusinessLine represents a business line/事业群
type BusinessLine struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Systems     []*System `json:"systems,omitempty"`
}

// System represents a system under a business line
type System struct {
	ID             string     `json:"id"`
	BusinessLineID string     `json:"business_line_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Projects       []*Project `json:"projects,omitempty"`
}

// Project represents a frontend or backend project
type Project struct {
	ID          string       `json:"id"`
	SystemID    string       `json:"system_id"`
	Name        string       `json:"name"`
	Type        ProjectType  `json:"type"`
	Description string       `json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Resources   []*Resource  `json:"resources,omitempty"`
}

// ProjectTypeDef defines a custom project type that can be created
type ProjectTypeDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// ProjectResource links a project to an external resource
type ProjectResource struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"project_id"`
	ResourceType ResourceType `json:"resource_type"`
	ResourceID   string       `json:"resource_id"`
	CreatedAt    time.Time    `json:"created_at"`
}

// ProjectPermission represents RBAC permission at any level
type ProjectPermission struct {
	ID             string     `json:"id"`
	Level          string     `json:"level"` // "project", "system", or "business_line"
	ProjectID      *string    `json:"project_id,omitempty"`
	SystemID       *string    `json:"system_id,omitempty"`
	BusinessLineID *string    `json:"business_line_id,omitempty"`
	Role           Role       `json:"role"`
	Subject        string     `json:"subject"` // LDAP user DN or group DN
	CreatedAt      time.Time  `json:"created_at"`
}

// Resource represents a linked resource with type info
type Resource struct {
	ID           string       `json:"id"`
	ResourceType ResourceType `json:"resource_type"`
	ResourceID   string       `json:"resource_id"`
	CreatedAt    time.Time    `json:"created_at"`
}

// FinOpsRow represents a row in the FinOps CSV export
type FinOpsRow struct {
	BusinessLine string `json:"business_line"`
	System       string `json:"system"`
	ProjectType  string `json:"project_type"`
	Project      string `json:"project"`
	ResourceType string `json:"resource_type"`
	Count        int    `json:"count"`
	Unit         string `json:"unit"`
}

// Pagination holds pagination metadata per API spec
type Pagination struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// PaginatedResponse wraps a list response with pagination
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// NewBusinessLine creates a new business line with generated ID
func NewBusinessLine(name, description string) *BusinessLine {
	now := time.Now()
	return &BusinessLine{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewSystem creates a new system with generated ID
func NewSystem(blID, name, description string) *System {
	now := time.Now()
	return &System{
		ID:             uuid.New().String(),
		BusinessLineID: blID,
		Name:           name,
		Description:    description,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewProject creates a new project with generated ID
func NewProject(sysID, name string, projType ProjectType, description string) *Project {
	now := time.Now()
	return &Project{
		ID:          uuid.New().String(),
		SystemID:    sysID,
		Name:        name,
		Type:        projType,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewProjectResource creates a new project resource link
func NewProjectResource(projID string, resType ResourceType, resID string) *ProjectResource {
	return &ProjectResource{
		ID:           uuid.New().String(),
		ProjectID:    projID,
		ResourceType: resType,
		ResourceID:   resID,
		CreatedAt:    time.Now(),
	}
}

// NewPermission creates a new permission
func NewPermission(level string, projectID, systemID, blID *string, role Role, subject string) *ProjectPermission {
	return &ProjectPermission{
		ID:             uuid.New().String(),
		Level:          level,
		ProjectID:      projectID,
		SystemID:       systemID,
		BusinessLineID: blID,
		Role:           role,
		Subject:        subject,
		CreatedAt:      time.Now(),
	}
}