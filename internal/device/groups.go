package device

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/database"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// DeviceGroup is the model for the device-groups feature. A
// group is a flat collection of device IDs (the spec does not
// require nested groups). The Device model carries a nullable
// FK back to this table.
type DeviceGroup struct {
	database.BaseModel
	Name        string `gorm:"column:name;size:128;not null;index" json:"name"`
	Description string `gorm:"column:description;size:512" json:"description,omitempty"`
}

// TableName pins the table name.
func (DeviceGroup) TableName() string { return "device_groups" }

// GroupRepository is the GORM-only data-access layer for
// DeviceGroup. Mirrors Repository's contract so the two
// services look identical to the handler.
type GroupRepository struct {
	db *gorm.DB
}

// NewGroupRepository builds a GroupRepository backed by db.
func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Create persists a new device group.
func (r *GroupRepository) Create(g *DeviceGroup) error {
	if err := r.db.Create(g).Error; err != nil {
		return fmt.Errorf("group.Create: %w", err)
	}
	return nil
}

// Get returns a single group by ID, or ErrGroupNotFound.
func (r *GroupRepository) Get(id string) (*DeviceGroup, error) {
	var g DeviceGroup
	if err := r.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrGroupNotFound)
		return nil, fmt.Errorf("group.Get: %w", err)
	}
	return &g, nil
}

// List returns every group, paged.
func (r *GroupRepository) List(limit, offset int) ([]DeviceGroup, int64, error) {
	q := r.db.Model(&DeviceGroup{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("group.List count: %w", err)
	}
	rows := []DeviceGroup{}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("group.List find: %w", err)
	}
	return rows, total, nil
}

// Update persists changes to a group.
func (r *GroupRepository) Update(g *DeviceGroup) error {
	res := r.db.Save(g)
	if res.Error != nil {
		return fmt.Errorf("group.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// Delete soft-deletes a group.
func (r *GroupRepository) Delete(id string) error {
	res := r.db.Delete(&DeviceGroup{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("group.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// ErrGroupNotFound is the typed sentinel for a missing group.
var ErrGroupNotFound = errors.New("device group: not found")

// IsGroupNotFound reports whether err is (or wraps)
// ErrGroupNotFound.
func IsGroupNotFound(err error) bool {
	return errors.Is(err, ErrGroupNotFound)
}

// GroupCreateInput is the request payload for GroupService.Create.
type GroupCreateInput struct {
	Name        string
	Description string
}

// GroupUpdateInput is the request payload for GroupService.Update.
type GroupUpdateInput struct {
	Name        *string
	Description *string
}

// GroupService is the business-logic layer for device groups.
type GroupService struct {
	repo *GroupRepository
}

// NewGroupService builds a GroupService.
func NewGroupService(repo *GroupRepository) *GroupService {
	return &GroupService{repo: repo}
}

// Create validates and persists a group.
func (s *GroupService) Create(in GroupCreateInput) (*DeviceGroup, error) {
	if in.Name == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	g := &DeviceGroup{Name: in.Name, Description: in.Description}
	if err := s.repo.Create(g); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create device group",
			Cause:   err,
		}
	}
	return g, nil
}

// Get returns a single group by ID.
func (s *GroupService) Get(id string) (*DeviceGroup, error) {
	g, err := s.repo.Get(id)
	if err != nil {
		if IsGroupNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device group %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load device group",
			Cause:   err,
		}
	}
	return g, nil
}

// List returns every group, paged.
func (s *GroupService) List(limit, offset int) ([]DeviceGroup, int64, error) {
	rows, total, err := s.repo.List(limit, offset)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list device groups",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Update persists changes to a group.
func (s *GroupService) Update(id string, in GroupUpdateInput) (*DeviceGroup, error) {
	g, err := s.repo.Get(id)
	if err != nil {
		if IsGroupNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device group %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load device group",
			Cause:   err,
		}
	}
	if in.Name != nil {
		if *in.Name == "" {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "name cannot be blank",
			}
		}
		g.Name = *in.Name
	}
	if in.Description != nil {
		g.Description = *in.Description
	}
	if err := s.repo.Update(g); err != nil {
		if IsGroupNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device group %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update device group",
			Cause:   err,
		}
	}
	return g, nil
}

// Delete soft-deletes a group.
func (s *GroupService) Delete(id string) error {
	if err := s.repo.Delete(id); err != nil {
		if IsGroupNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("device group %q not found", id),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete device group",
			Cause:   err,
		}
	}
	return nil
}

// GroupHandler is the HTTP layer for device groups.
type GroupHandler struct {
	svc *GroupService
}

// NewGroupHandler builds a GroupHandler.
func NewGroupHandler(svc *GroupService) *GroupHandler {
	return &GroupHandler{svc: svc}
}

// Register attaches the device-group routes to the supplied
// router group.
func (h *GroupHandler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewDevices)
	writeP := perms(rbac.PermissionManageDeviceGroups)
	r.GET("/device-groups", viewP, h.List)
	r.POST("/device-groups", writeP, h.Create)
	r.GET("/device-groups/:id", viewP, h.Get)
	r.PUT("/device-groups/:id", writeP, h.Update)
	r.DELETE("/device-groups/:id", writeP, h.Delete)
}

// groupRequest is the wire shape for the POST/PUT endpoints.
type groupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// List handles GET /device-groups.
func (h *GroupHandler) List(c *gin.Context) {
	rows, total, err := h.svc.List(20, 0)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	page := &contracts.Pagination{Total: total, Limit: 20, Offset: 0, HasMore: false}
	handler.WriteList(c.Writer, rows, page)
}

// Create handles POST /device-groups.
func (h *GroupHandler) Create(c *gin.Context) {
	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	g, err := h.svc.Create(GroupCreateInput{Name: req.Name, Description: req.Description})
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, g)
}

// Get handles GET /device-groups/:id.
func (h *GroupHandler) Get(c *gin.Context) {
	id := c.Param("id")
	g, err := h.svc.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, g)
}

// Update handles PUT /device-groups/:id.
func (h *GroupHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req groupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	in := GroupUpdateInput{}
	if req.Name != "" {
		name := req.Name
		in.Name = &name
	}
	if req.Description != "" {
		desc := req.Description
		in.Description = &desc
	}
	g, err := h.svc.Update(id, in)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, g)
}

// Delete handles DELETE /device-groups/:id.
func (h *GroupHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}
