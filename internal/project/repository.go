package project

import (
	"strings"
	"time"

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

// ProjectType CRUD
func (r *Repository) ListProjectTypes() ([]*ProjectTypeDef, error) {
	var types []GORMProjectType
	if err := r.db.Order("name").Find(&types).Error; err != nil {
		return nil, err
	}
	result := make([]*ProjectTypeDef, len(types))
	for i := range types {
		result[i] = &ProjectTypeDef{
			ID:          types[i].ID,
			Name:        types[i].Name,
			Description: types[i].Description,
			Color:       types[i].Color,
		}
	}
	return result, nil
}

func (r *Repository) GetProjectType(id string) (*ProjectTypeDef, error) {
	var pt GORMProjectType
	if err := r.db.First(&pt, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ProjectTypeDef{
		ID:          pt.ID,
		Name:        pt.Name,
		Description: pt.Description,
		Color:       pt.Color,
	}, nil
}

func (r *Repository) CreateProjectType(pt *ProjectTypeDef) error {
	gpt := GORMProjectType{
		ID:          pt.ID,
		Name:        pt.Name,
		Description: pt.Description,
		Color:       pt.Color,
	}
	return r.db.Create(&gpt).Error
}

func (r *Repository) UpdateProjectType(pt *ProjectTypeDef) error {
	return r.db.Model(&GORMProjectType{}).Where("id = ?", pt.ID).Updates(map[string]interface{}{
		"name":        pt.Name,
		"description": pt.Description,
		"color":       pt.Color,
	}).Error
}

func (r *Repository) DeleteProjectType(id string) error {
	var count int64
	if err := r.db.Model(&GORMProject{}).Where("type = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return gorm.ErrInvalidDB
	}
	return r.db.Delete(&GORMProjectType{}, "id = ?", id).Error
}

func (r *Repository) ProjectTypeExists(id string) (bool, error) {
	var count int64
	if err := r.db.Model(&GORMProjectType{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ValidateProjectType(t string) bool {
	exists, err := r.ProjectTypeExists(t)
	if err != nil {
		return false
	}
	return exists
}

// BusinessLine CRUD
func (r *Repository) CreateBusinessLine(bl *BusinessLine) error {
	gbl := GORMBusinessLine{
		ID:          bl.ID,
		Name:        bl.Name,
		Description: bl.Description,
	}
	return r.db.Create(&gbl).Error
}

func (r *Repository) GetBusinessLine(id string) (*BusinessLine, error) {
	var gbl GORMBusinessLine
	if err := r.db.First(&gbl, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.fromGORMBusinessLine(&gbl), nil
}

func (r *Repository) ListBusinessLines(page, perPage int) ([]*BusinessLine, int, error) {
	var total int64
	if err := r.db.Model(&GORMBusinessLine{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	var gbls []GORMBusinessLine
	if err := r.db.Order("name").Limit(perPage).Offset(offset).Find(&gbls).Error; err != nil {
		return nil, 0, err
	}
	result := make([]*BusinessLine, len(gbls))
	for i := range gbls {
		result[i] = r.fromGORMBusinessLine(&gbls[i])
	}
	return result, int(total), nil
}

func (r *Repository) UpdateBusinessLine(bl *BusinessLine) error {
	return r.db.Model(&GORMBusinessLine{}).Where("id = ?", bl.ID).Updates(map[string]interface{}{
		"name":        bl.Name,
		"description": bl.Description,
		"updated_at":  time.Now(),
	}).Error
}

func (r *Repository) DeleteBusinessLine(id string) error {
	return r.db.Delete(&GORMBusinessLine{}, "id = ?", id).Error
}

func (r *Repository) GetBusinessLineWithSystems(id string) (*BusinessLine, error) {
	var gbl GORMBusinessLine
	if err := r.db.Preload("Systems").First(&gbl, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	bl := r.fromGORMBusinessLine(&gbl)
	bl.Systems = r.systemsFromGORM(gbl.Systems)
	return bl, nil
}

// System CRUD
func (r *Repository) CreateSystem(s *System) error {
	gs := GORMSystem{
		ID:             s.ID,
		BusinessLineID: s.BusinessLineID,
		Name:           s.Name,
		Description:    s.Description,
	}
	return r.db.Create(&gs).Error
}

func (r *Repository) GetSystem(id string) (*System, error) {
	var gs GORMSystem
	if err := r.db.First(&gs, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.fromGORMSystem(&gs), nil
}

func (r *Repository) ListSystemsByBusinessLine(blID string) ([]*System, error) {
	var systems []GORMSystem
	if err := r.db.Where("business_line_id = ?", blID).Order("name").Find(&systems).Error; err != nil {
		return nil, err
	}
	return r.systemsFromGORM(systems), nil
}

func (r *Repository) ListSystems(page, perPage int) ([]*System, int, error) {
	var total int64
	if err := r.db.Model(&GORMSystem{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	var systems []GORMSystem
	if err := r.db.Order("name").Limit(perPage).Offset(offset).Find(&systems).Error; err != nil {
		return nil, 0, err
	}
	return r.systemsFromGORM(systems), int(total), nil
}

func (r *Repository) UpdateSystem(s *System) error {
	return r.db.Model(&GORMSystem{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
		"name":        s.Name,
		"description": s.Description,
		"updated_at":  time.Now(),
	}).Error
}

func (r *Repository) DeleteSystem(id string) error {
	return r.db.Delete(&GORMSystem{}, "id = ?", id).Error
}

func (r *Repository) GetSystemWithProjects(id string) (*System, error) {
	var gs GORMSystem
	if err := r.db.Preload("Projects").First(&gs, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	sys := r.fromGORMSystem(&gs)
	sys.Projects = r.projectsFromGORM(gs.Projects)
	return sys, nil
}

// Project CRUD
func (r *Repository) CreateProject(p *Project) error {
	gp := GORMProject{
		ID:          p.ID,
		SystemID:    p.SystemID,
		Name:        p.Name,
		Type:        p.Type,
		Description: p.Description,
	}
	return r.db.Create(&gp).Error
}

func (r *Repository) GetProject(id string) (*Project, error) {
	var gp GORMProject
	if err := r.db.First(&gp, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.fromGORMProject(&gp), nil
}

func (r *Repository) ListProjectsBySystem(sysID string) ([]*Project, error) {
	var projects []GORMProject
	if err := r.db.Where("system_id = ?", sysID).Order("name").Find(&projects).Error; err != nil {
		return nil, err
	}
	return r.projectsFromGORM(projects), nil
}

func (r *Repository) ListProjects(page, perPage int) ([]*Project, int, error) {
	var total int64
	if err := r.db.Model(&GORMProject{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	var projects []GORMProject
	if err := r.db.Order("name").Limit(perPage).Offset(offset).Find(&projects).Error; err != nil {
		return nil, 0, err
	}
	return r.projectsFromGORM(projects), int(total), nil
}

func (r *Repository) UpdateProject(p *Project) error {
	return r.db.Model(&GORMProject{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"name":        p.Name,
		"type":        p.Type,
		"description": p.Description,
		"updated_at":  time.Now(),
	}).Error
}

func (r *Repository) DeleteProject(id string) error {
	return r.db.Delete(&GORMProject{}, "id = ?", id).Error
}

func (r *Repository) GetProjectWithResources(id string) (*Project, error) {
	var gp GORMProject
	if err := r.db.Preload("Resources").First(&gp, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	proj := r.fromGORMProject(&gp)
	proj.Resources = r.resourcesFromGORM(gp.Resources)
	return proj, nil
}

// ProjectResource CRUD
func (r *Repository) CreateProjectResource(pr *ProjectResource) error {
	gpr := GORMResource{
		ID:           pr.ID,
		ProjectID:    pr.ProjectID,
		ResourceType: pr.ResourceType,
		ResourceID:   pr.ResourceID,
	}
	return r.db.Create(&gpr).Error
}

func (r *Repository) ListProjectResources(projectID string) ([]*Resource, error) {
	var gresources []GORMResource
	if err := r.db.Where("project_id = ?", projectID).Order("resource_type").Find(&gresources).Error; err != nil {
		return nil, err
	}
	return r.resourcesFromGORM(gresources), nil
}

func (r *Repository) DeleteProjectResource(projectID, resourceID string) error {
	return r.db.Where("project_id = ? AND resource_id = ?", projectID, resourceID).Delete(&GORMResource{}).Error
}

func (r *Repository) GetProjectResource(projectID, resourceID string) (*ProjectResource, error) {
	var gpr GORMResource
	if err := r.db.Where("project_id = ? AND resource_id = ?", projectID, resourceID).First(&gpr).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ProjectResource{
		ID:           gpr.ID,
		ProjectID:    gpr.ProjectID,
		ResourceType: gpr.ResourceType,
		ResourceID:   gpr.ResourceID,
		CreatedAt:    gpr.CreatedAt,
	}, nil
}

// Permission CRUD
func (r *Repository) CreatePermission(p *ProjectPermission) error {
	gp := GORMPermission{
		ID:             p.ID,
		Level:          p.Level,
		ProjectID:      p.ProjectID,
		SystemID:       p.SystemID,
		BusinessLineID: p.BusinessLineID,
		Role:           p.Role,
		Subject:        p.Subject,
	}
	return r.db.Create(&gp).Error
}

func (r *Repository) ListPermissionsByProject(projectID string) ([]*ProjectPermission, error) {
	var perms []GORMPermission
	if err := r.db.Where("project_id = ?", projectID).Find(&perms).Error; err != nil {
		return nil, err
	}
	return r.permissionsFromGORM(perms), nil
}

func (r *Repository) ListPermissionsBySubject(subject string) ([]*ProjectPermission, error) {
	var perms []GORMPermission
	if err := r.db.Where("subject = ?", subject).Find(&perms).Error; err != nil {
		return nil, err
	}
	return r.permissionsFromGORM(perms), nil
}

func (r *Repository) DeletePermission(id string) error {
	return r.db.Delete(&GORMPermission{}, "id = ?", id).Error
}

func (r *Repository) CheckPermission(subject string, projectID string) (Role, error) {
	var perm GORMPermission
	err := r.db.Where("project_id = ? AND subject = ?", projectID, subject).First(&perm).Error
	if err == nil {
		return perm.Role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return "", err
	}

	// Walk up: system -> business_line
	p, err := r.GetProject(projectID)
	if err != nil || p == nil {
		return "", nil
	}
	sys, err := r.GetSystem(p.SystemID)
	if err != nil || sys == nil {
		return "", nil
	}
	err = r.db.Where("system_id = ? AND subject = ?", p.SystemID, subject).First(&perm).Error
	if err == nil {
		return perm.Role, nil
	}
	if err != gorm.ErrRecordNotFound {
		return "", err
	}

	err = r.db.Where("business_line_id = ? AND subject = ?", sys.BusinessLineID, subject).First(&perm).Error
	if err == nil {
		return perm.Role, nil
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	return "", err
}

// FinOps report
func (r *Repository) GetFinOpsData(period string) ([]FinOpsRow, error) {
	type finOpsResult struct {
		BusinessLine string
		System       string
		ProjectType  string
		Project      string
		ResourceType string
		Count        int64
		Unit         string
	}
	var results []finOpsResult
	err := r.db.Raw(`
		SELECT
			bl.name as business_line,
			s.name as system,
			p.type as project_type,
			p.name as project,
			pr.resource_type,
			COUNT(*) as count,
			CASE
				WHEN pr.resource_type = 'device' THEN 'nodes'
				WHEN pr.resource_type = 'physical_host' THEN 'nodes'
				WHEN pr.resource_type = 'pipeline' THEN 'pipelines'
				WHEN pr.resource_type = 'log_source' THEN 'sources'
				WHEN pr.resource_type = 'alert_channel' THEN 'channels'
				ELSE 'units'
			END as unit
		FROM project_resources pr
		JOIN projects p ON pr.project_id = p.id
		JOIN systems s ON p.system_id = s.id
		JOIN business_lines bl ON s.business_line_id = bl.id
		GROUP BY bl.name, s.name, p.type, p.name, pr.resource_type
		ORDER BY bl.name, s.name, p.type, p.name, pr.resource_type
	`).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	finOpsRows := make([]FinOpsRow, len(results))
	for i, r := range results {
		finOpsRows[i] = FinOpsRow{
			BusinessLine: r.BusinessLine,
			System:       r.System,
			ProjectType:  r.ProjectType,
			Project:      r.Project,
			ResourceType: r.ResourceType,
			Count:        int(r.Count),
			Unit:         r.Unit,
		}
	}
	return finOpsRows, nil
}

func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetSystemByProjectID returns the system for a given project
func (r *Repository) GetSystemByProjectID(projectID string) (*System, error) {
	p, err := r.GetProject(projectID)
	if err != nil || p == nil {
		return nil, err
	}
	return r.GetSystem(p.SystemID)
}

// GetBusinessLineBySystemID returns the business line for a given system
func (r *Repository) GetBusinessLineBySystemID(systemID string) (*BusinessLine, error) {
	s, err := r.GetSystem(systemID)
	if err != nil || s == nil {
		return nil, err
	}
	return r.GetBusinessLine(s.BusinessLineID)
}

// StringInSlice checks if a string is in a slice
func stringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ValidateProjectType validates project type
func ValidateProjectType(t string) bool {
	return stringInSlice(t, []string{string(ProjectTypeFrontend), string(ProjectTypeBackend)})
}

// ValidateResourceType validates resource type
func ValidateResourceType(t string) bool {
	return stringInSlice(t, []string{
		string(ResourceTypeDevice),
		string(ResourceTypePipeline),
		string(ResourceTypeLogSource),
		string(ResourceTypeAlertChannel),
		string(ResourceTypePhysicalHost),
	})
}

// ValidateRole validates role
func ValidateRole(r string) bool {
	return stringInSlice(r, []string{string(RoleViewer), string(RoleEditor), string(RoleAdmin)})
}

// ValidateLevel validates permission level
func ValidateLevel(l string) bool {
	return strings.HasPrefix(l, "project") || strings.HasPrefix(l, "system") || strings.HasPrefix(l, "business_line")
}

// AuditLog CRUD
func (r *Repository) CreateAuditLog(log *AuditLog) error {
	gl := AuditLog{
		ID:         log.ID,
		Timestamp:  log.Timestamp,
		Username:   log.Username,
		Action:     log.Action,
		EntityType: log.EntityType,
		EntityID:   log.EntityID,
		EntityName: log.EntityName,
		Changes:    log.Changes,
		OldValue:   log.OldValue,
		NewValue:   log.NewValue,
		IPAddress:  log.IPAddress,
	}
	return r.db.Create(&gl).Error
}

func (r *Repository) ListAuditLogs(entityType, entityID, username string, limit, offset int) ([]*AuditLog, int, error) {
	query := r.db.Model(&AuditLog{})
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	if entityID != "" {
		query = query.Where("entity_id = ?", entityID)
	}
	if username != "" {
		query = query.Where("username = ?", username)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []AuditLog
	if err := query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	result := make([]*AuditLog, len(logs))
	for i := range logs {
		result[i] = &logs[i]
	}
	return result, int(total), nil
}

func (r *Repository) GetAuditLog(id string) (*AuditLog, error) {
	var log AuditLog
	if err := r.db.First(&log, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// Conversion helpers
func (r *Repository) fromGORMBusinessLine(gbl *GORMBusinessLine) *BusinessLine {
	if gbl == nil {
		return nil
	}
	return &BusinessLine{
		ID:          gbl.ID,
		Name:        gbl.Name,
		Description: gbl.Description,
		CreatedAt:   gbl.CreatedAt,
		UpdatedAt:   gbl.UpdatedAt,
	}
}

func (r *Repository) fromGORMSystem(gs *GORMSystem) *System {
	if gs == nil {
		return nil
	}
	return &System{
		ID:             gs.ID,
		BusinessLineID: gs.BusinessLineID,
		Name:           gs.Name,
		Description:    gs.Description,
		CreatedAt:      gs.CreatedAt,
		UpdatedAt:      gs.UpdatedAt,
	}
}

func (r *Repository) systemsFromGORM(systems []GORMSystem) []*System {
	result := make([]*System, len(systems))
	for i := range systems {
		result[i] = r.fromGORMSystem(&systems[i])
	}
	return result
}

func (r *Repository) fromGORMProject(gp *GORMProject) *Project {
	if gp == nil {
		return nil
	}
	return &Project{
		ID:          gp.ID,
		SystemID:    gp.SystemID,
		Name:        gp.Name,
		Type:        gp.Type,
		Description: gp.Description,
		CreatedAt:   gp.CreatedAt,
		UpdatedAt:   gp.UpdatedAt,
	}
}

func (r *Repository) projectsFromGORM(projects []GORMProject) []*Project {
	result := make([]*Project, len(projects))
	for i := range projects {
		result[i] = r.fromGORMProject(&projects[i])
	}
	return result
}

func (r *Repository) resourcesFromGORM(resources []GORMResource) []*Resource {
	result := make([]*Resource, len(resources))
	for i := range resources {
		result[i] = &Resource{
			ID:           resources[i].ID,
			ResourceType: resources[i].ResourceType,
			ResourceID:   resources[i].ResourceID,
			CreatedAt:    resources[i].CreatedAt,
		}
	}
	return result
}

func (r *Repository) permissionsFromGORM(perms []GORMPermission) []*ProjectPermission {
	result := make([]*ProjectPermission, len(perms))
	for i := range perms {
		result[i] = &ProjectPermission{
			ID:             perms[i].ID,
			Level:          perms[i].Level,
			ProjectID:      perms[i].ProjectID,
			SystemID:       perms[i].SystemID,
			BusinessLineID: perms[i].BusinessLineID,
			Role:           perms[i].Role,
			Subject:        perms[i].Subject,
			CreatedAt:      perms[i].CreatedAt,
		}
	}
	return result
}