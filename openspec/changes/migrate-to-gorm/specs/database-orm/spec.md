# database-orm

## ADDED Requirements

### Requirement: GORM Database Connection
The system SHALL use GORM as the ORM layer for all database operations.

#### Scenario: Database connection initialization
- **WHEN** application starts
- **THEN** GORM connects to PostgreSQL using DSN from config
- **AND** AutoMigrate runs to create/update schema

**Implementation:**
- `pkg/database/gorm.go` - `NewGORM()` function
- `GORMConfig` struct with Host, Port, User, Password, Name, SSLMode, MaxConnections, ConnMaxLifetime
- Returns `*gorm.DB` with configured connection pool

### Requirement: Device Model
The system SHALL define Device model using GORM tags.

#### Scenario: Device CRUD operations
- **WHEN** creating a device
- **THEN** GORM inserts record and returns populated struct
- **AND** timestamps (CreatedAt, UpdatedAt) are auto-managed

**Implementation:**
- `internal/device/models.go` - `GORMDevice` struct with gorm.Model embedded
- `TableName()` returns "devices"
- `StringMap` type for Labels with Value/Scan for JSONB
- `JSONMap` type for Config/Metadata with Value/Scan for JSONB

#### Scenario: Device pagination
- **WHEN** listing devices with pagination
- **THEN** GORM returns paginated results with total count
- **AND** ordering by created_at DESC

**Implementation:**
- `ListPaginated()` uses `db.Model(&GORMDevice{}).Count()` for total
- `db.Order("created_at DESC").Limit(limit).Offset(offset).Find()`

### Requirement: Project Hierarchy Models
The system SHALL define BusinessLine, System, Project models with GORM relationships.

#### Scenario: BusinessLine with Systems
- **WHEN** loading BusinessLine
- **THEN** GORM Preload fetches associated Systems
- **AND** JSON response includes nested Systems array

**Implementation:**
- `GORMBusinessLine` with `Systems []GORMSystem` and `gorm:"foreignKey:BusinessLineID"`
- `GetBusinessLineWithSystems()` uses `db.Preload("Systems")`

#### Scenario: Project with Resources
- **WHEN** loading Project with resources
- **THEN** GORM Preload fetches associated ProjectResources
- **AND** JSON response includes nested Resources array

**Implementation:**
- `GORMProject` with `Resources []GORMResource` and `gorm:"foreignKey:ProjectID"`
- `GetProjectWithResources()` uses `db.Preload("Resources")`

### Requirement: JSONB Field Handling
The system SHALL handle JSONB fields using GORM's JSON serializer.

#### Scenario: Device Labels
- **WHEN** storing device with labels map
- **THEN** GORM serializes to JSONB automatically
- **AND** deserialization returns correct map on read

**Implementation:**
- `StringMap` implements `driver.Valuer` and `sql.Scanner` for JSON serialization
- GORM tag: `gorm:"type:jsonb;serializer:json"`

### Requirement: Permission Model with Constraints
The system SHALL implement ProjectPermission with CHECK constraints.

#### Scenario: Permission level validation
- **WHEN** creating permission with invalid level
- **THEN** database CHECK constraint rejects the insert
- **OR** GORM validation returns error

**Implementation:**
- `GORMPermission` struct validates at application level
- `ValidateRole()` checks against RoleViewer, RoleEditor, RoleAdmin

### Requirement: Audit Log Model
The system SHALL implement AuditLog for tracking changes.

#### Scenario: Create audit log
- **WHEN** entity is modified
- **THEN** AuditLog record is created with timestamp, username, action, changes

#### Scenario: Query audit logs
- **WHEN** querying audit logs with filters
- **THEN** GORM builds dynamic query with WHERE clauses
- **AND** results are paginated

**Implementation:**
- `AuditLog` struct in `internal/project/audit.go`
- `ListAuditLogs()` uses chainable `Where()` for dynamic filters
- `Order("timestamp DESC").Limit(limit).Offset(offset)`

## Implemented Models

| Model | Table | File |
|-------|-------|------|
| GORMDevice | devices | internal/device/models.go |
| GORMBusinessLine | business_lines | internal/project/models.go |
| GORMProjectType | project_types | internal/project/models.go |
| GORMSystem | systems | internal/project/models.go |
| GORMProject | projects | internal/project/models.go |
| GORMResource | project_resources | internal/project/models.go |
| GORMPermission | project_permissions | internal/project/models.go |
| AuditLog | audit_logs | internal/project/audit.go |

## Repository Changes

| Module | Old Type | New Type |
|--------|----------|----------|
| Device | `*sql.DB` | `*gorm.DB` |
| Project | `*sql.DB` | `*gorm.DB` |

## Removed Dependencies
- `github.com/lib/pq` - replaced by `gorm.io/driver/postgres`

## Added Dependencies
- `gorm.io/gorm v1.31.1`
- `gorm.io/driver/postgres v1.6.0`
- `github.com/jinzhu/inflection v1.0.0`
