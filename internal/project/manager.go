package project

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/devops-toolkit/internal/auth"
	"github.com/gorilla/mux"
)

type Manager struct {
	repo *Repository
}

func NewManager(repo *Repository) *Manager {
	return &Manager{repo: repo}
}

func NewManagerWithDSN(dsn string) (*Manager, error) {
	repo, err := NewRepository(dsn)
	if err != nil {
		return nil, err
	}
	return &Manager{repo: repo}, nil
}

func (m *Manager) parsePagination(r *http.Request) (page, perPage int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	return page, perPage
}

func (m *Manager) paginatedResponse(data interface{}, total, page, perPage int) *PaginatedResponse {
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return &PaginatedResponse{
		Data: data,
		Pagination: Pagination{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			Pages:   pages,
		},
	}
}

// BusinessLine handlers
func (m *Manager) ListBusinessLinesHTTP(w http.ResponseWriter, r *http.Request) {
	page, perPage := m.parsePagination(r)
	bls, total, err := m.repo.ListBusinessLines(page, perPage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(bls, total, page, perPage))
}

func (m *Manager) CreateBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	bl := NewBusinessLine(input.Name, input.Description)
	if err := m.repo.CreateBusinessLine(bl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bl == nil {
		http.Error(w, "business line not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bl)
}

func (m *Manager) UpdateBusinessLineHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	bl, err := m.repo.GetBusinessLine(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bl == nil {
		http.Error(w, "business line not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldName := bl.Name
	oldDesc := bl.Description
	if input.Name != "" {
		bl.Name = input.Name
	}
	bl.Description = input.Description
	if err := m.repo.UpdateBusinessLine(bl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bl == nil {
		http.Error(w, "business line not found", http.StatusNotFound)
		return
	}
	if err := m.repo.DeleteBusinessLine(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "business_line", id, bl.Name)
		auditLog.OldValue = bl.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// System handlers
func (m *Manager) ListSystemsHTTP(w http.ResponseWriter, r *http.Request) {
	blID := mux.Vars(r)["bl_id"]
	if blID != "" {
		systems, err := m.repo.ListSystemsByBusinessLine(blID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(systems)
		return
	}
	page, perPage := m.parsePagination(r)
	systems, total, err := m.repo.ListSystems(page, perPage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(systems, total, page, perPage))
}

func (m *Manager) CreateSystemHTTP(w http.ResponseWriter, r *http.Request) {
	blID := mux.Vars(r)["bl_id"]
	if blID == "" {
		http.Error(w, "business line ID is required", http.StatusBadRequest)
		return
	}
	bl, err := m.repo.GetBusinessLine(blID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bl == nil {
		http.Error(w, "business line not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	sys := NewSystem(blID, input.Name, input.Description)
	if err := m.repo.CreateSystem(sys); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sys == nil {
		http.Error(w, "system not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sys)
}

func (m *Manager) UpdateSystemHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	sys, err := m.repo.GetSystem(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sys == nil {
		http.Error(w, "system not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldName := sys.Name
	oldDesc := sys.Description
	if input.Name != "" {
		sys.Name = input.Name
	}
	sys.Description = input.Description
	if err := m.repo.UpdateSystem(sys); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sys == nil {
		http.Error(w, "system not found", http.StatusNotFound)
		return
	}
	if err := m.repo.DeleteSystem(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
		auditLog := NewAuditLog(user.Username, ActionDelete, "system", id, sys.Name)
		auditLog.OldValue = sys.Name
		auditLog.IPAddress = getClientIP(r)
		m.repo.CreateAuditLog(auditLog)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Project handlers
func (m *Manager) ListProjectsHTTP(w http.ResponseWriter, r *http.Request) {
	sysID := mux.Vars(r)["sys_id"]
	if sysID != "" {
		projects, err := m.repo.ListProjectsBySystem(sysID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
		return
	}
	page, perPage := m.parsePagination(r)
	projects, total, err := m.repo.ListProjects(page, perPage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(projects, total, page, perPage))
}

func (m *Manager) CreateProjectHTTP(w http.ResponseWriter, r *http.Request) {
	sysID := mux.Vars(r)["sys_id"]
	if sysID == "" {
		http.Error(w, "system ID is required", http.StatusBadRequest)
		return
	}
	sys, err := m.repo.GetSystem(sysID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sys == nil {
		http.Error(w, "system not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name        string      `json:"name"`
		Type        ProjectType `json:"type"`
		Description string      `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if !ValidateProjectType(string(input.Type)) {
		http.Error(w, "invalid project type", http.StatusBadRequest)
		return
	}
	proj := NewProject(sysID, input.Name, input.Type, input.Description)
	if err := m.repo.CreateProject(proj); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if proj == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proj)
}

func (m *Manager) UpdateProjectHTTP(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	proj, err := m.repo.GetProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if proj == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	var input struct {
		Name        string      `json:"name"`
		Type        ProjectType `json:"type"`
		Description string      `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldName := proj.Name
	oldType := proj.Type
	oldDesc := proj.Description
	if input.Name != "" {
		proj.Name = input.Name
	}
	if input.Type != "" && ValidateProjectType(string(input.Type)) {
		proj.Type = input.Type
	}
	proj.Description = input.Description
	if err := m.repo.UpdateProject(proj); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if proj == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if err := m.repo.DeleteProject(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !ValidateResourceType(string(input.ResourceType)) {
		http.Error(w, "invalid resource type", http.StatusBadRequest)
		return
	}
	if input.ResourceID == "" {
		http.Error(w, "resource_id is required", http.StatusBadRequest)
		return
	}
	proj, err := m.repo.GetProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if proj == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	pr := NewProjectResource(id, input.ResourceType, input.ResourceID)
	if err := m.repo.CreateProjectResource(pr); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			http.Error(w, "resource already linked", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pr == nil {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	proj, err := m.repo.GetProject(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if proj == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if err := m.repo.DeleteProjectResource(id, resourceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !ValidateRole(string(input.Role)) {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	if input.Subject == "" {
		http.Error(w, "subject is required", http.StatusBadRequest)
		return
	}
	level := strings.ToLower(input.Level)
	if level == "project" {
		input.BusinessLineID = nil
		input.SystemID = nil
	} else if level == "system" {
		if input.SystemID == nil || *input.SystemID == "" {
			http.Error(w, "system_id is required for system-level permission", http.StatusBadRequest)
			return
		}
		input.BusinessLineID = nil
	} else if level == "business_line" {
		if input.BusinessLineID == nil || *input.BusinessLineID == "" {
			http.Error(w, "business_line_id is required for business_line-level permission", http.StatusBadRequest)
			return
		}
		input.SystemID = nil
	} else {
		http.Error(w, "level must be project, system, or business_line", http.StatusBadRequest)
		return
	}

	projID := id
	perm := NewPermission(level, &projID, input.SystemID, input.BusinessLineID, input.Role, input.Subject)
	if err := m.repo.CreatePermission(perm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log
	if user := auth.GetUserFromContext(r.Context()); user != nil {
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
				if user := auth.GetUserFromContext(r.Context()); user != nil {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// FinOps export
func (m *Manager) ExportFinOpsHTTP(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	rows, err := m.repo.GetFinOpsData(period)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=finops-%s.csv", period))
	csvWriter := csv.NewWriter(w)
	headers := []string{"Business Line", "System", "Project Type", "Project", "Resource Type", "Count", "Unit"}
	if err := csvWriter.Write(headers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	logs, total, err := m.repo.ListAuditLogs(entityType, entityID, username, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.paginatedResponse(logs, total, offset/limit+1, limit))
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