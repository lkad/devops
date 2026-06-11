package device

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/database"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ConfigurationTemplate is a Jinja2-style configuration template
// that can be applied to one or more devices. The renderer is
// out of scope for this package — we only store and retrieve
// the body. The spec's "Apply template to device" scenario is
// integration-level and lives outside this module.
type ConfigurationTemplate struct {
	database.BaseModel
	Name        string `gorm:"column:name;size:128;not null;index" json:"name"`
	Description string `gorm:"column:description;size:512" json:"description,omitempty"`
	// Body is the raw template text.
	Body string `gorm:"column:body;type:text" json:"body"`
}

// TableName pins the table name.
func (ConfigurationTemplate) TableName() string { return "configuration_templates" }

// ErrTemplateNotFound is the typed sentinel for a missing
// template.
var ErrTemplateNotFound = errors.New("template: not found")

// IsTemplateNotFound reports whether err is (or wraps)
// ErrTemplateNotFound.
func IsTemplateNotFound(err error) bool {
	return errors.Is(err, ErrTemplateNotFound)
}

// TemplateRepository is the GORM-only data-access layer for
// ConfigurationTemplate.
type TemplateRepository struct {
	db *gorm.DB
}

// NewTemplateRepository builds a TemplateRepository backed by db.
func NewTemplateRepository(db *gorm.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

// Create persists a new template.
func (r *TemplateRepository) Create(t *ConfigurationTemplate) error {
	if err := r.db.Create(t).Error; err != nil {
		return fmt.Errorf("template.Create: %w", err)
	}
	return nil
}

// Get returns a single template by ID, or ErrTemplateNotFound.
func (r *TemplateRepository) Get(id string) (*ConfigurationTemplate, error) {
	var t ConfigurationTemplate
	if err := r.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, database.MapNotFound(err, ErrTemplateNotFound)
		return nil, fmt.Errorf("template.Get: %w", err)
	}
	return &t, nil
}

// List returns every template, paged.
func (r *TemplateRepository) List(limit, offset int) ([]ConfigurationTemplate, int64, error) {
	q := r.db.Model(&ConfigurationTemplate{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("template.List count: %w", err)
	}
	rows := []ConfigurationTemplate{}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("template.List find: %w", err)
	}
	return rows, total, nil
}

// Update persists changes to a template.
func (r *TemplateRepository) Update(t *ConfigurationTemplate) error {
	res := r.db.Save(t)
	if res.Error != nil {
		return fmt.Errorf("template.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

// Delete soft-deletes a template.
func (r *TemplateRepository) Delete(id string) error {
	res := r.db.Delete(&ConfigurationTemplate{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("template.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

// TemplateCreateInput is the request payload for
// TemplateService.Create.
type TemplateCreateInput struct {
	Name        string
	Description string
	Body        string
}

// TemplateUpdateInput is the request payload for
// TemplateService.Update.
type TemplateUpdateInput struct {
	Name        *string
	Description *string
	Body        *string
}

// TemplateService is the business-logic layer for configuration
// templates.
type TemplateService struct {
	repo  *TemplateRepository
	audit *audit.Service
}

// NewTemplateService builds a TemplateService. The audit
// service is optional (nil means "no audit emission");
// production always wires a real service so v0.2.0.0
// P0 #3 audit-trail coverage holds.
func NewTemplateService(repo *TemplateRepository, auditSvc ...*audit.Service) *TemplateService {
	var a *audit.Service
	if len(auditSvc) > 0 {
		a = auditSvc[0]
	}
	return &TemplateService{repo: repo, audit: a}
}

// Create validates and persists a template. The audit
// emission (device_template.create) is best-effort; the
// context is variadic so existing test rig keeps
// compiling.
func (s *TemplateService) Create(in TemplateCreateInput, ctxArg ...context.Context) (*ConfigurationTemplate, error) {
	ctx := s.ctxOrBackground(ctxArg)
	if in.Name == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	t := &ConfigurationTemplate{Name: in.Name, Description: in.Description, Body: in.Body}
	if err := s.repo.Create(t); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create template",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionCreate,
			ResourceType: audit.ResourceDevice,
			ResourceID:   t.ID,
			Metadata:     audit.JSONMap{"template": true, "name": t.Name},
		})
	}
	return t, nil
}

// Get returns a single template by ID.
func (s *TemplateService) Get(id string) (*ConfigurationTemplate, error) {
	t, err := s.repo.Get(id)
	if err != nil {
		if IsTemplateNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("template %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load template",
			Cause:   err,
		}
	}
	return t, nil
}

// List returns every template, paged.
func (s *TemplateService) List(limit, offset int) ([]ConfigurationTemplate, int64, error) {
	rows, total, err := s.repo.List(limit, offset)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list templates",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Update persists changes to a template. The audit
// emission (device_template.update) is best-effort; the
// context is variadic so existing test rig keeps
// compiling.
func (s *TemplateService) Update(id string, in TemplateUpdateInput, ctxArg ...context.Context) (*ConfigurationTemplate, error) {
	ctx := s.ctxOrBackground(ctxArg)
	t, err := s.repo.Get(id)
	if err != nil {
		if IsTemplateNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("template %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load template",
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
		t.Name = *in.Name
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Body != nil {
		t.Body = *in.Body
	}
	if err := s.repo.Update(t); err != nil {
		if IsTemplateNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("template %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update template",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionUpdate,
			ResourceType: audit.ResourceDevice,
			ResourceID:   t.ID,
			Metadata:     audit.JSONMap{"template": true, "name": t.Name},
		})
	}
	return t, nil
}

// Delete soft-deletes a template. The audit emission
// (device_template.delete) is best-effort; the context is
// variadic so existing test rig keeps compiling.
func (s *TemplateService) Delete(id string, ctxArg ...context.Context) error {
	ctx := s.ctxOrBackground(ctxArg)
	if err := s.repo.Delete(id); err != nil {
		if IsTemplateNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("template %q not found", id),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete template",
			Cause:   err,
		}
	}
	if s.audit != nil {
		s.audit.RecordAction(ctx, audit.RecordActionInput{
			Action:       audit.ActionDelete,
			ResourceType: audit.ResourceDevice,
			ResourceID:   id,
			Metadata:     audit.JSONMap{"template": true},
		})
	}
	return nil
}

// ctxOrBackground returns the first supplied context, or
// context.Background() when none was supplied.
func (s *TemplateService) ctxOrBackground(args []context.Context) context.Context {
	if len(args) > 0 && args[0] != nil {
		return args[0]
	}
	return context.Background()
}

// TemplateHandler is the HTTP layer for configuration templates.
type TemplateHandler struct {
	svc *TemplateService
}

// NewTemplateHandler builds a TemplateHandler.
func NewTemplateHandler(svc *TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

// Register attaches the configuration-template routes to the
// supplied router group.
func (h *TemplateHandler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewDevices)
	writeP := perms(rbac.PermissionManageConfigurationTemplates)
	r.GET("/configuration-templates", viewP, h.List)
	r.POST("/configuration-templates", writeP, h.Create)
	r.GET("/configuration-templates/:id", viewP, h.Get)
	r.PUT("/configuration-templates/:id", writeP, h.Update)
	r.DELETE("/configuration-templates/:id", writeP, h.Delete)
}

// templateRequest is the wire shape for the POST/PUT endpoints.
type templateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// List handles GET /configuration-templates.
func (h *TemplateHandler) List(c *gin.Context) {
	rows, total, err := h.svc.List(20, 0)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	page := &contracts.Pagination{Total: total, Limit: 20, Offset: 0, HasMore: false}
	handler.WriteList(c.Writer, rows, page)
}

// Create handles POST /configuration-templates.
func (h *TemplateHandler) Create(c *gin.Context) {
	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	t, err := h.svc.Create(TemplateCreateInput{
		Name:        req.Name,
		Description: req.Description,
		Body:        req.Body,
	})
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteCreated(c.Writer, t)
}

// Get handles GET /configuration-templates/:id.
func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")
	t, err := h.svc.Get(id)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, t)
}

// Update handles PUT /configuration-templates/:id.
func (h *TemplateHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req templateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	in := TemplateUpdateInput{}
	if req.Name != "" {
		name := req.Name
		in.Name = &name
	}
	if req.Description != "" {
		desc := req.Description
		in.Description = &desc
	}
	if req.Body != "" {
		body := req.Body
		in.Body = &body
	}
	t, err := h.svc.Update(id, in)
	if err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, t)
}

// Delete handles DELETE /configuration-templates/:id.
func (h *TemplateHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		handler.WriteAPIError(c.Writer, err)
		return
	}
	handler.WriteNoContent(c.Writer)
}
