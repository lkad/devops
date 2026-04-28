package project

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// UserProvider defines the interface for getting user information from request context
type UserProvider interface {
	GetUserFromRequest(r *http.Request) *User
}

// User represents authenticated user info needed for audit logging
type User struct {
	Username string `json:"username"`
}

// AuthUserProvider implements UserProvider by extracting user from request context
// using auth package's GetUserFromContext. This bridges the gap and allows
// the project package to remain decoupled from auth implementation details.
type AuthUserProvider struct {
	GetUserFromRequest func(r *http.Request) *User
}

type Manager struct {
	repo         *Repository
	userProvider UserProvider
}

func NewManager(repo *Repository, userProvider UserProvider) *Manager {
	return &Manager{repo: repo, userProvider: userProvider}
}

func NewManagerWithDB(db *gorm.DB, userProvider UserProvider) *Manager {
	repo := NewRepository(db)
	return &Manager{repo: repo, userProvider: userProvider}
}

func (m *Manager) parsePagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (m *Manager) paginatedResponse(data interface{}, total, limit, offset int) *PaginatedResponse {
	dataLen := 0
	if data != nil {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Slice {
			dataLen = v.Len()
		}
	}
	hasMore := dataLen > 0 && offset+dataLen < total
	return &PaginatedResponse{
		Data: data,
		Pagination: Pagination{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: hasMore,
		},
	}
}

// ProjectType handlers
func (m *Manager) ListProjectTypesHTTP(w http.ResponseWriter, r *http.Request) {
	types, err := m.repo.ListProjectTypes()
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types)
}

func (m *Manager) CreateProjectTypeHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if input.ID == "" || input.Name == "" {
		apierror.ValidationError(w, "id and name are required")
		return
	}
	// Validate ID format (lowercase alphanumeric and hyphens only)
	for _, c := range input.ID {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			apierror.ValidationError(w, "id must be lowercase alphanumeric with optional hyphens")
			return
		}
	}
	if input.Color == "" {
		input.Color = "#64748b"
	}
	pt := &ProjectTypeDef{
		ID:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Color:       input.Color,
	}
	if err := m.repo.CreateProjectType(pt); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pt)
}

func (m *Manager) UpdateProjectTypeHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	existing, err := m.repo.GetProjectType(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if existing == nil {
		apierror.NotFound(w, "project type not found")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if input.Name != "" {
		existing.Name = input.Name
	}
	if input.Description != "" {
		existing.Description = input.Description
	}
	if input.Color != "" {
		existing.Color = input.Color
	}
	if err := m.repo.UpdateProjectType(existing); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (m *Manager) DeleteProjectTypeHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "frontend" || id == "backend" {
		apierror.ValidationError(w, "cannot delete default project types")
		return
	}
	if err := m.repo.DeleteProjectType(id); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BusinessLine handlers
func (m *Manager) ListBusinessLinesHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := m.parsePagination(r)
	page := offset/limit + 1
	bls, total, err := m.repo.ListBusinessLines(page, limit)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(bls, total, limit, offset))
}

func (m *Manager) CreateBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if input.Name == "" {
		apierror.ValidationError(w, "name is required")
		return
	}
	bl := NewBusinessLine(input.Name, input.Description)
	if err := m.repo.CreateBusinessLine(bl); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionCreate, "business_line", bl.ID, bl.Name)
		auditLog.NewValue = input.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bl)
}

func (m *Manager) GetBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	bl, err := m.repo.GetBusinessLineWithSystems(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if bl == nil {
		apierror.NotFound(w, "business line not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bl)
}

func (m *Manager) UpdateBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	bl, err := m.repo.GetBusinessLine(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if bl == nil {
		apierror.NotFound(w, "business line not found")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	oldName := bl.Name
	oldDesc := bl.Description
	if input.Name != "" {
		bl.Name = input.Name
	}
	bl.Description = input.Description
	if err := m.repo.UpdateBusinessLine(bl); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		changes := fmt.Sprintf("name: %s -> %s, description: %s -> %s", oldName, bl.Name, oldDesc, bl.Description)
		auditLog := NewAuditLog(user.Username, ActionUpdate, "business_line", bl.ID, bl.Name)
		auditLog.Changes = changes
		auditLog.OldValue = oldName
		auditLog.NewValue = bl.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bl)
}

func (m *Manager) DeleteBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	bl, err := m.repo.GetBusinessLine(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if bl == nil {
		apierror.NotFound(w, "business line not found")
		return
	}
	if err := m.repo.DeleteBusinessLine(id); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "business_line", id, bl.Name)
		auditLog.OldValue = bl.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// System handlers
func (m *Manager) ListSystemsHTTP(w http.ResponseWriter, r *http.Request) {
	blID := mux.Vars(r)["id"]
	if blID != "" {
		systems, err := m.repo.ListSystemsByBusinessLine(blID)
		if err != nil {
			apierror.InternalErrorFromErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(systems)
		return
	}
	limit, offset := m.parsePagination(r)
	page := offset/limit + 1
	systems, total, err := m.repo.ListSystems(page, limit)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(systems, total, limit, offset))
}

func (m *Manager) CreateSystemHTTP(w http.ResponseWriter, r *http.Request) {
	blID := mux.Vars(r)["id"]
	if blID == "" {
		apierror.ValidationError(w, "business line ID is required")
		return
	}
	bl, err := m.repo.GetBusinessLine(blID)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if bl == nil {
		apierror.NotFound(w, "business line not found")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if input.Name == "" {
		apierror.ValidationError(w, "name is required")
		return
	}
	sys := NewSystem(blID, input.Name, input.Description)
	if err := m.repo.CreateSystem(sys); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionCreate, "system", sys.ID, sys.Name)
		auditLog.NewValue = input.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sys)
}

func (m *Manager) GetSystemHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	sys, err := m.repo.GetSystemWithProjects(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if sys == nil {
		apierror.NotFound(w, "system not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sys)
}

func (m *Manager) UpdateSystemHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	sys, err := m.repo.GetSystem(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if sys == nil {
		apierror.NotFound(w, "system not found")
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	oldName := sys.Name
	oldDesc := sys.Description
	if input.Name != "" {
		sys.Name = input.Name
	}
	sys.Description = input.Description
	if err := m.repo.UpdateSystem(sys); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		changes := fmt.Sprintf("name: %s -> %s, description: %s -> %s", oldName, sys.Name, oldDesc, sys.Description)
		auditLog := NewAuditLog(user.Username, ActionUpdate, "system", sys.ID, sys.Name)
		auditLog.Changes = changes
		auditLog.OldValue = oldName
		auditLog.NewValue = sys.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sys)
}

func (m *Manager) DeleteSystemHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	sys, err := m.repo.GetSystem(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if sys == nil {
		apierror.NotFound(w, "system not found")
		return
	}
	if err := m.repo.DeleteSystem(id); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "system", id, sys.Name)
		auditLog.OldValue = sys.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Project handlers
func (m *Manager) ListProjectsHTTP(w http.ResponseWriter, r *http.Request) {
	sysID := mux.Vars(r)["id"]
	if sysID != "" {
		projects, err := m.repo.ListProjectsBySystem(sysID)
		if err != nil {
			apierror.InternalErrorFromErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
		return
	}
	limit, offset := m.parsePagination(r)
	page := offset/limit + 1
	projects, total, err := m.repo.ListProjects(page, limit)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(projects, total, limit, offset))
}

func (m *Manager) CreateProjectHTTP(w http.ResponseWriter, r *http.Request) {
	sysID := mux.Vars(r)["id"]
	if sysID == "" {
		apierror.ValidationError(w, "system ID is required")
		return
	}
	sys, err := m.repo.GetSystem(sysID)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if sys == nil {
		apierror.NotFound(w, "system not found")
		return
	}
	var input struct {
		Name        string      `json:"name"`
		Type        ProjectType `json:"type"`
		Description string      `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if input.Name == "" {
		apierror.ValidationError(w, "name is required")
		return
	}
	if !m.repo.ValidateProjectType(string(input.Type)) {
		apierror.ValidationError(w, "invalid project type")
		return
	}
	proj := NewProject(sysID, input.Name, input.Type, input.Description)
	if err := m.repo.CreateProject(proj); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionCreate, "project", proj.ID, proj.Name)
		auditLog.NewValue = fmt.Sprintf("%s (%s)", input.Name, input.Type)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(proj)
}

func (m *Manager) GetProjectHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	proj, err := m.repo.GetProjectWithResources(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if proj == nil {
		apierror.NotFound(w, "project not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proj)
}

func (m *Manager) UpdateProjectHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	proj, err := m.repo.GetProject(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if proj == nil {
		apierror.NotFound(w, "project not found")
		return
	}
	var input struct {
		Name        string      `json:"name"`
		Type        ProjectType `json:"type"`
		Description string      `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	oldName := proj.Name
	oldType := proj.Type
	oldDesc := proj.Description
	if input.Name != "" {
		proj.Name = input.Name
	}
	if input.Type != "" && m.repo.ValidateProjectType(string(input.Type)) {
		proj.Type = input.Type
	}
	proj.Description = input.Description
	if err := m.repo.UpdateProject(proj); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		changes := fmt.Sprintf("name: %s -> %s, type: %s -> %s, description: %s -> %s",
			oldName, proj.Name, oldType, proj.Type, oldDesc, proj.Description)
		auditLog := NewAuditLog(user.Username, ActionUpdate, "project", proj.ID, proj.Name)
		auditLog.Changes = changes
		auditLog.OldValue = fmt.Sprintf("%s (%s)", oldName, oldType)
		auditLog.NewValue = fmt.Sprintf("%s (%s)", proj.Name, proj.Type)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proj)
}

func (m *Manager) DeleteProjectHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	proj, err := m.repo.GetProject(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if proj == nil {
		apierror.NotFound(w, "project not found")
		return
	}
	if err := m.repo.DeleteProject(id); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "project", id, proj.Name)
		auditLog.OldValue = fmt.Sprintf("%s (%s)", proj.Name, proj.Type)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Resource linking handlers
func (m *Manager) ListProjectResourcesHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	resources, err := m.repo.ListProjectResources(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resources)
}

func (m *Manager) LinkResourceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var input struct {
		ResourceType ResourceType `json:"resource_type"`
		ResourceID   string       `json:"resource_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if !ValidateResourceType(string(input.ResourceType)) {
		apierror.ValidationError(w, "invalid resource type")
		return
	}
	if input.ResourceID == "" {
		apierror.ValidationError(w, "resource_id is required")
		return
	}
	proj, err := m.repo.GetProject(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if proj == nil {
		apierror.NotFound(w, "project not found")
		return
	}
	pr := NewProjectResource(id, input.ResourceType, input.ResourceID)
	if err := m.repo.CreateProjectResource(pr); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			apierror.Conflict(w, "resource already linked")
			return
		}
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionCreate, "resource_link", pr.ID, proj.Name)
		auditLog.NewValue = fmt.Sprintf("%s: %s", input.ResourceType, input.ResourceID)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pr)
}

func (m *Manager) UnlinkResourceHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	resourceID := mux.Vars(r)["resource_id"]
	pr, err := m.repo.GetProjectResource(id, resourceID)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if pr == nil {
		apierror.NotFound(w, "resource not found")
		return
	}
	proj, err := m.repo.GetProject(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	if proj == nil {
		apierror.NotFound(w, "project not found")
		return
	}
	if err := m.repo.DeleteProjectResource(id, resourceID); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "resource_link", pr.ID, proj.Name)
		auditLog.OldValue = fmt.Sprintf("%s: %s", pr.ResourceType, pr.ResourceID)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Permission handlers
func (m *Manager) ListProjectPermissionsHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	perms, err := m.repo.ListPermissionsByProject(id)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perms)
}

func (m *Manager) GrantPermissionHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var input struct {
		Level          string   `json:"level"`
		Role           Role     `json:"role"`
		Subject        string   `json:"subject"`
		BusinessLineID *string  `json:"business_line_id,omitempty"`
		SystemID       *string  `json:"system_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	if !ValidateRole(string(input.Role)) {
		apierror.ValidationError(w, "invalid role")
		return
	}
	if input.Subject == "" {
		apierror.ValidationError(w, "subject is required")
		return
	}
	level := strings.ToLower(input.Level)
	if level == "project" {
		input.BusinessLineID = nil
		input.SystemID = nil
	} else if level == "system" {
		if input.SystemID == nil || *input.SystemID == "" {
			apierror.ValidationError(w, "system_id is required for system-level permission")
			return
		}
		input.BusinessLineID = nil
	} else if level == "business_line" {
		if input.BusinessLineID == nil || *input.BusinessLineID == "" {
			apierror.ValidationError(w, "business_line_id is required for business_line-level permission")
			return
		}
		input.SystemID = nil
	} else {
		apierror.ValidationError(w, "level must be project, system, or business_line")
		return
	}

	projID := id
	perm := NewPermission(level, &projID, input.SystemID, input.BusinessLineID, input.Role, input.Subject)
	if err := m.repo.CreatePermission(perm); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}

	// Audit log
	if user := m.userProvider.GetUserFromRequest(r); user != nil {
		auditLog := NewAuditLog(user.Username, ActionCreate, "permission", perm.ID, fmt.Sprintf("%s on %s", input.Role, level))
		auditLog.NewValue = fmt.Sprintf("%s: %s (%s)", input.Role, input.Subject, level)
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(perm)
}

func (m *Manager) RevokePermissionHTTP(w http.ResponseWriter, r *http.Request) {
	permID := mux.Vars(r)["perm_id"]

	// Get permission details before deletion for audit log
	perms, err := m.repo.ListPermissionsBySubject("")
	if err == nil {
		for _, p := range perms {
			if p.ID == permID {
				if user := m.userProvider.GetUserFromRequest(r); user != nil {
					auditLog := NewAuditLog(user.Username, ActionDelete, "permission", permID, fmt.Sprintf("%s on %s", p.Role, p.Level))
					auditLog.OldValue = fmt.Sprintf("%s: %s (%s)", p.Role, p.Subject, p.Level)
					auditLog.IPAddress = getClientIP(r)
					m.repo.CreateAuditLog(auditLog)
				}
				break
			}
		}
	}

	if err := m.repo.DeletePermission(permID); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// FinOps export
func (m *Manager) ExportFinOpsHTTP(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	rows, err := m.repo.GetFinOpsData(period)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=finops-%s.csv", period))
	csvWriter := csv.NewWriter(w)
	headers := []string{"Business Line", "System", "Project Type", "Project", "Resource Type", "Count", "Unit"}
	if err := csvWriter.Write(headers); err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	for _, row := range rows {
		record := []string{
			row.BusinessLine,
			row.System,
			row.ProjectType,
			row.Project,
			row.ResourceType,
			strconv.Itoa(row.Count),
			row.Unit,
		}
		if err := csvWriter.Write(record); err != nil {
			apierror.InternalErrorFromErr(w, err)
			return
		}
	}
	csvWriter.Flush()
}

// CheckPermission checks if subject has at least the given role on a project
func (m *Manager) CheckPermission(subject string, projectID string, requiredRole Role) (bool, error) {
	if m.repo == nil {
		return false, fmt.Errorf("repository not initialized")
	}
	role, err := m.repo.CheckPermission(subject, projectID)
	if err != nil {
		return false, err
	}
	if role == "" {
		return false, nil
	}
	// Role hierarchy: admin > editor > viewer
	roleWeight := map[Role]int{
		RoleViewer: 1,
		RoleEditor: 2,
		RoleAdmin:  3,
	}
	return roleWeight[role] >= roleWeight[requiredRole], nil
}

// AuditLog handlers
func (m *Manager) ListAuditLogsHTTP(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	username := r.URL.Query().Get("username")
	limit, offset := m.parsePagination(r)

	logs, total, err := m.repo.ListAuditLogs(entityType, entityID, username, limit, offset)
	if err != nil {
		apierror.InternalErrorFromErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(logs, total, limit, offset))
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}