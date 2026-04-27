# project-hierarchy

## ADDED Requirements

### Requirement: Business Line Management
The system SHALL support Business Line CRUD operations.

#### Scenario: Create Business Line
- **WHEN** user sends POST /api/org/business-lines with name and description
- **THEN** system creates Business Line and returns 201 with ID

#### Scenario: List Business Lines
- **WHEN** user requests GET /api/org/business-lines
- **THEN** system returns array of all Business Lines

#### Scenario: Get Business Line
- **WHEN** user requests GET /api/org/business-lines/:id
- **THEN** system returns Business Line with nested systems

#### Scenario: Update Business Line
- **WHEN** user sends PUT /api/org/business-lines/:id with modified data
- **THEN** system updates Business Line and returns 200

#### Scenario: Delete Business Line
- **WHEN** user sends DELETE /api/org/business-lines/:id
- **THEN** system deletes Business Line and all child systems and projects

### Requirement: System Management
The system SHALL support System CRUD under Business Lines.

#### Scenario: Create System
- **WHEN** user sends POST /api/org/business-lines/:bl_id/systems with name
- **THEN** system creates System under Business Line

#### Scenario: List Systems under Business Line
- **WHEN** user requests GET /api/org/business-lines/:bl_id/systems
- **THEN** system returns array of Systems

#### Scenario: Get System
- **WHEN** user requests GET /api/org/systems/:id
- **THEN** system returns System with nested projects

#### Scenario: Update System
- **WHEN** user sends PUT /api/org/systems/:id with modified data
- **THEN** system updates System

#### Scenario: Delete System
- **WHEN** user sends DELETE /api/org/systems/:id
- **THEN** system deletes System and all child projects

### Requirement: Project Management
The system SHALL support Project CRUD under Systems.

#### Scenario: Create Project
- **WHEN** user sends POST /api/org/systems/:sys_id/projects with name and type
- **THEN** system creates Project under System

#### Scenario: List Projects under System
- **WHEN** user requests GET /api/org/systems/:sys_id/projects
- **THEN** system returns array of Projects

#### Scenario: Get Project
- **WHEN** user requests GET /api/org/projects/:id
- **THEN** system returns Project with resource links

#### Scenario: Update Project
- **WHEN** user sends PUT /api/org/projects/:id with modified data
- **THEN** system updates Project

#### Scenario: Delete Project
- **WHEN** user sends DELETE /api/org/projects/:id
- **THEN** system deletes Project and resource links

### Requirement: Resource Linking
The system SHALL link DevOps resources to Projects.

#### Scenario: Link resource to project
- **WHEN** user sends POST /api/org/projects/:id/resources with resource type and ID
- **THEN** system creates resource link

#### Scenario: List project resources
- **WHEN** user requests GET /api/org/projects/:id/resources
- **THEN** system returns all linked resources

#### Scenario: Unlink resource
- **WHEN** user sends DELETE /api/org/projects/:id/resources/:resource_id
- **THEN** system removes resource link

### Requirement: Permission Management
The system SHALL support local RBAC permissions on projects.

#### Scenario: Grant viewer permission
- **WHEN** user sends POST /api/org/projects/:id/permissions with user and role=viewer
- **THEN** system grants viewer permission

#### Scenario: Grant editor permission
- **WHEN** user sends POST /api/org/projects/:id/permissions with user and role=editor
- **THEN** system grants editor permission

#### Scenario: Revoke permission
- **WHEN** user sends DELETE /api/org/permissions/:perm_id
- **THEN** system removes permission

### Requirement: Permission Inheritance
The system SHALL cascade permissions down the hierarchy.

#### Scenario: Business Line editor can edit all children
- **WHEN** user with editor permission on Business Line accesses child System
- **THEN** system allows edit

### Requirement: FinOps Report Export
The system SHALL export resource usage CSV for billing.

#### Scenario: Export FinOps report
- **WHEN** user requests GET /api/org/reports/finops?period=2026-04
- **THEN** system returns CSV with Business Line, System, Project, Resource Type, Count, Unit

### Requirement: Audit Logging
All CRUD operations on project hierarchy SHALL be logged to audit trail.
See [audit-logging](../audit-logging/spec.md) for full requirements.

### Requirement: Query Audit Logs
Audit log query endpoint is provided via the audit-logging module.
See [audit-logging](../audit-logging/spec.md) for query parameters and response format.
