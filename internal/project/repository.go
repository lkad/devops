package project

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(dsn string) (*Repository, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS business_lines (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS systems (
		id TEXT PRIMARY KEY,
		business_line_id TEXT NOT NULL REFERENCES business_lines(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		system_id TEXT NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS project_resources (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		UNIQUE(project_id, resource_type, resource_id)
	);

	CREATE TABLE IF NOT EXISTS project_permissions (
		id TEXT PRIMARY KEY,
		level TEXT NOT NULL CHECK (level IN ('project', 'system', 'business_line')),
		project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
		system_id TEXT REFERENCES systems(id) ON DELETE CASCADE,
		business_line_id TEXT REFERENCES business_lines(id) ON DELETE CASCADE,
		role TEXT NOT NULL CHECK (role IN ('viewer', 'editor', 'admin')),
		subject TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		CONSTRAINT exactly_one_level CHECK (
			(CASE WHEN level = 'project' THEN 1 ELSE 0 END +
			 CASE WHEN level = 'system' THEN 1 ELSE 0 END +
			 CASE WHEN level = 'business_line' THEN 1 ELSE 0 END) = 1
		),
		CONSTRAINT proper_fk_check CHECK (
			(level = 'project' AND project_id IS NOT NULL) OR
			(level = 'system' AND system_id IS NOT NULL) OR
			(level = 'business_line' AND business_line_id IS NOT NULL)
		)
	);

	CREATE INDEX IF NOT EXISTS idx_systems_business_line ON systems(business_line_id);
	CREATE INDEX IF NOT EXISTS idx_projects_system ON projects(system_id);
	CREATE INDEX IF NOT EXISTS idx_project_resources_project ON project_resources(project_id);
	CREATE INDEX IF NOT EXISTS idx_project_resources_type ON project_resources(resource_type);
	CREATE INDEX IF NOT EXISTS idx_project_permissions_subject ON project_permissions(subject);
	CREATE INDEX IF NOT EXISTS idx_project_permissions_level ON project_permissions(level);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		timestamp TIMESTAMP DEFAULT NOW(),
		username TEXT NOT NULL,
		action TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		entity_name TEXT,
		changes TEXT,
		old_value TEXT,
		new_value TEXT,
		ip_address TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);
	CREATE INDEX IF NOT EXISTS idx_audit_username ON audit_logs(username);
	`
	_, err := r.db.Exec(schema)
	return err
}

// BusinessLine CRUD
func (r *Repository) CreateBusinessLine(bl *BusinessLine) error {
	query := `INSERT INTO business_lines (id, name, description) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(query, bl.ID, bl.Name, bl.Description)
	return err
}

func (r *Repository) GetBusinessLine(id string) (*BusinessLine, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM business_lines WHERE id = $1`
	bl := &BusinessLine{}
	err := r.db.QueryRow(query, id).Scan(&bl.ID, &bl.Name, &bl.Description, &bl.CreatedAt, &bl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return bl, nil
}

func (r *Repository) ListBusinessLines(page, perPage int) ([]*BusinessLine, int, error) {
	countQuery := `SELECT COUNT(*) FROM business_lines`
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	query := `SELECT id, name, description, created_at, updated_at FROM business_lines ORDER BY name LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var results []*BusinessLine
	for rows.Next() {
		bl := &BusinessLine{}
		if err := rows.Scan(&bl.ID, &bl.Name, &bl.Description, &bl.CreatedAt, &bl.UpdatedAt); err != nil {
			return nil, 0, err
		}
		results = append(results, bl)
	}
	return results, total, nil
}

func (r *Repository) UpdateBusinessLine(bl *BusinessLine) error {
	query := `UPDATE business_lines SET name = $2, description = $3, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, bl.ID, bl.Name, bl.Description)
	return err
}

func (r *Repository) DeleteBusinessLine(id string) error {
	query := `DELETE FROM business_lines WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *Repository) GetBusinessLineWithSystems(id string) (*BusinessLine, error) {
	bl, err := r.GetBusinessLine(id)
	if err != nil || bl == nil {
		return bl, err
	}
	systems, err := r.ListSystemsByBusinessLine(id)
	if err != nil {
		return nil, err
	}
	bl.Systems = systems
	return bl, nil
}

// System CRUD
func (r *Repository) CreateSystem(s *System) error {
	query := `INSERT INTO systems (id, business_line_id, name, description) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(query, s.ID, s.BusinessLineID, s.Name, s.Description)
	return err
}

func (r *Repository) GetSystem(id string) (*System, error) {
	query := `SELECT id, business_line_id, name, description, created_at, updated_at FROM systems WHERE id = $1`
	s := &System{}
	err := r.db.QueryRow(query, id).Scan(&s.ID, &s.BusinessLineID, &s.Name, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) ListSystemsByBusinessLine(blID string) ([]*System, error) {
	query := `SELECT id, business_line_id, name, description, created_at, updated_at FROM systems WHERE business_line_id = $1 ORDER BY name`
	rows, err := r.db.Query(query, blID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*System
	for rows.Next() {
		s := &System{}
		if err := rows.Scan(&s.ID, &s.BusinessLineID, &s.Name, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, nil
}

func (r *Repository) ListSystems(page, perPage int) ([]*System, int, error) {
	countQuery := `SELECT COUNT(*) FROM systems`
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	query := `SELECT id, business_line_id, name, description, created_at, updated_at FROM systems ORDER BY name LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var results []*System
	for rows.Next() {
		s := &System{}
		if err := rows.Scan(&s.ID, &s.BusinessLineID, &s.Name, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		results = append(results, s)
	}
	return results, total, nil
}

func (r *Repository) UpdateSystem(s *System) error {
	query := `UPDATE systems SET name = $2, description = $3, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, s.ID, s.Name, s.Description)
	return err
}

func (r *Repository) DeleteSystem(id string) error {
	query := `DELETE FROM systems WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *Repository) GetSystemWithProjects(id string) (*System, error) {
	s, err := r.GetSystem(id)
	if err != nil || s == nil {
		return s, err
	}
	projects, err := r.ListProjectsBySystem(id)
	if err != nil {
		return nil, err
	}
	s.Projects = projects
	return s, nil
}

// Project CRUD
func (r *Repository) CreateProject(p *Project) error {
	query := `INSERT INTO projects (id, system_id, name, type, description) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(query, p.ID, p.SystemID, p.Name, p.Type, p.Description)
	return err
}

func (r *Repository) GetProject(id string) (*Project, error) {
	query := `SELECT id, system_id, name, type, description, created_at, updated_at FROM projects WHERE id = $1`
	p := &Project{}
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.SystemID, &p.Name, &p.Type, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) ListProjectsBySystem(sysID string) ([]*Project, error) {
	query := `SELECT id, system_id, name, type, description, created_at, updated_at FROM projects WHERE system_id = $1 ORDER BY name`
	rows, err := r.db.Query(query, sysID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Project
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(&p.ID, &p.SystemID, &p.Name, &p.Type, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}

func (r *Repository) ListProjects(page, perPage int) ([]*Project, int, error) {
	countQuery := `SELECT COUNT(*) FROM projects`
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	query := `SELECT id, system_id, name, type, description, created_at, updated_at FROM projects ORDER BY name LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var results []*Project
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(&p.ID, &p.SystemID, &p.Name, &p.Type, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		results = append(results, p)
	}
	return results, total, nil
}

func (r *Repository) UpdateProject(p *Project) error {
	query := `UPDATE projects SET name = $2, type = $3, description = $4, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, p.ID, p.Name, p.Type, p.Description)
	return err
}

func (r *Repository) DeleteProject(id string) error {
	query := `DELETE FROM projects WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *Repository) GetProjectWithResources(id string) (*Project, error) {
	p, err := r.GetProject(id)
	if err != nil || p == nil {
		return p, err
	}
	resources, err := r.ListProjectResources(id)
	if err != nil {
		return nil, err
	}
	p.Resources = resources
	return p, nil
}

// ProjectResource CRUD
func (r *Repository) CreateProjectResource(pr *ProjectResource) error {
	query := `INSERT INTO project_resources (id, project_id, resource_type, resource_id) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(query, pr.ID, pr.ProjectID, pr.ResourceType, pr.ResourceID)
	return err
}

func (r *Repository) ListProjectResources(projectID string) ([]*Resource, error) {
	query := `SELECT id, resource_type, resource_id, created_at FROM project_resources WHERE project_id = $1 ORDER BY resource_type`
	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Resource
	for rows.Next() {
		pr := &Resource{}
		if err := rows.Scan(&pr.ID, &pr.ResourceType, &pr.ResourceID, &pr.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, pr)
	}
	return results, nil
}

func (r *Repository) DeleteProjectResource(projectID, resourceID string) error {
	query := `DELETE FROM project_resources WHERE project_id = $1 AND resource_id = $2`
	_, err := r.db.Exec(query, projectID, resourceID)
	return err
}

func (r *Repository) GetProjectResource(projectID, resourceID string) (*ProjectResource, error) {
	query := `SELECT id, project_id, resource_type, resource_id, created_at FROM project_resources WHERE project_id = $1 AND resource_id = $2`
	pr := &ProjectResource{}
	err := r.db.QueryRow(query, projectID, resourceID).Scan(&pr.ID, &pr.ProjectID, &pr.ResourceType, &pr.ResourceID, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return pr, nil
}

// Permission CRUD
func (r *Repository) CreatePermission(p *ProjectPermission) error {
	query := `INSERT INTO project_permissions (id, level, project_id, system_id, business_line_id, role, subject) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(query, p.ID, p.Level, p.ProjectID, p.SystemID, p.BusinessLineID, p.Role, p.Subject)
	return err
}

func (r *Repository) ListPermissionsByProject(projectID string) ([]*ProjectPermission, error) {
	query := `SELECT id, level, project_id, system_id, business_line_id, role, subject, created_at FROM project_permissions WHERE project_id = $1`
	return r.queryPermissions(query, projectID)
}

func (r *Repository) ListPermissionsBySubject(subject string) ([]*ProjectPermission, error) {
	query := `SELECT id, level, project_id, system_id, business_line_id, role, subject, created_at FROM project_permissions WHERE subject = $1`
	return r.queryPermissions(query, subject)
}

func (r *Repository) queryPermissions(query string, arg interface{}) ([]*ProjectPermission, error) {
	rows, err := r.db.Query(query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*ProjectPermission
	for rows.Next() {
		p := &ProjectPermission{}
		if err := rows.Scan(&p.ID, &p.Level, &p.ProjectID, &p.SystemID, &p.BusinessLineID, &p.Role, &p.Subject, &p.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}

func (r *Repository) DeletePermission(id string) error {
	query := `DELETE FROM project_permissions WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *Repository) CheckPermission(subject string, projectID string) (Role, error) {
	// Check project-level permission
	query := `SELECT role FROM project_permissions WHERE project_id = $1 AND subject = $2 LIMIT 1`
	var role string
	err := r.db.QueryRow(query, projectID, subject).Scan(&role)
	if err == nil {
		return Role(role), nil
	}
	if err != sql.ErrNoRows {
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
	query = `SELECT role FROM project_permissions WHERE system_id = $1 AND subject = $2 LIMIT 1`
	err = r.db.QueryRow(query, p.SystemID, subject).Scan(&role)
	if err == nil {
		return Role(role), nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	query = `SELECT role FROM project_permissions WHERE business_line_id = $1 AND subject = $2 LIMIT 1`
	err = r.db.QueryRow(query, sys.BusinessLineID, subject).Scan(&role)
	if err == nil {
		return Role(role), nil
	}
	if err == sql.ErrNoRows {
		return "", nil
	}
	return "", err
}

// FinOps report
func (r *Repository) GetFinOpsData(period string) ([]FinOpsRow, error) {
	query := `
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
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []FinOpsRow
	for rows.Next() {
		row := FinOpsRow{}
		if err := rows.Scan(&row.BusinessLine, &row.System, &row.ProjectType, &row.Project, &row.ResourceType, &row.Count, &row.Unit); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

// Helper to parse JSON
func parseJSON(b []byte, v interface{}) error {
	return json.Unmarshal(b, v)
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
	query := `INSERT INTO audit_logs (id, timestamp, username, action, entity_type, entity_id, entity_name, changes, old_value, new_value, ip_address)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.db.Exec(query, log.ID, log.Timestamp, log.Username, log.Action, log.EntityType, log.EntityID, log.EntityName, log.Changes, log.OldValue, log.NewValue, log.IPAddress)
	return err
}

func (r *Repository) ListAuditLogs(entityType, entityID, username string, limit, offset int) ([]*AuditLog, int, error) {
	// Build count query
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if entityType != "" {
		countQuery += ` AND entity_type = $` + string(rune('0'+argIdx))
		args = append(args, entityType)
		argIdx++
	}
	if entityID != "" {
		countQuery += ` AND entity_id = $` + string(rune('0'+argIdx))
		args = append(args, entityID)
		argIdx++
	}
	if username != "" {
		countQuery += ` AND username = $` + string(rune('0'+argIdx))
		args = append(args, username)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Build select query
	selectQuery := `SELECT id, timestamp, username, action, entity_type, entity_id, entity_name, changes, old_value, new_value, ip_address
	                FROM audit_logs WHERE 1=1`
	args = []interface{}{}
	argIdx = 1

	if entityType != "" {
		selectQuery += ` AND entity_type = $` + string(rune('0'+argIdx))
		args = append(args, entityType)
		argIdx++
	}
	if entityID != "" {
		selectQuery += ` AND entity_id = $` + string(rune('0'+argIdx))
		args = append(args, entityID)
		argIdx++
	}
	if username != "" {
		selectQuery += ` AND username = $` + string(rune('0'+argIdx))
		args = append(args, username)
		argIdx++
	}

	selectQuery += ` ORDER BY timestamp DESC LIMIT $` + string(rune('0'+argIdx)) + ` OFFSET $` + string(rune('0'+argIdx+1))
	args = append(args, limit, offset)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*AuditLog
	for rows.Next() {
		log := &AuditLog{}
		if err := rows.Scan(&log.ID, &log.Timestamp, &log.Username, &log.Action, &log.EntityType, &log.EntityID, &log.EntityName, &log.Changes, &log.OldValue, &log.NewValue, &log.IPAddress); err != nil {
			return nil, 0, err
		}
		results = append(results, log)
	}
	return results, total, nil
}

func (r *Repository) GetAuditLog(id string) (*AuditLog, error) {
	query := `SELECT id, timestamp, username, action, entity_type, entity_id, entity_name, changes, old_value, new_value, ip_address
	          FROM audit_logs WHERE id = $1`
	log := &AuditLog{}
	err := r.db.QueryRow(query, id).Scan(&log.ID, &log.Timestamp, &log.Username, &log.Action, &log.EntityType, &log.EntityID, &log.EntityName, &log.Changes, &log.OldValue, &log.NewValue, &log.IPAddress)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return log, nil
}