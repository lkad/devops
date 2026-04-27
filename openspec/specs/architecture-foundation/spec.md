# architecture-foundation

## ADDED Requirements

### Requirement: Package Structure
Each module in `internal/` SHALL follow the same package structure with handler, service, repository, and models files.

#### Scenario: Standard module structure
- **WHEN** a new module is created
- **THEN** it contains handler.go, service.go, repository.go, and models.go

### Requirement: Handler Layer Responsibilities
Handlers SHALL only handle HTTP concerns: request parsing, validation, response formatting, and error handling.

#### Scenario: Handler delegates to service
- **WHEN** handler receives valid HTTP request
- **THEN** handler calls service method and formats response
- **AND** handler does not contain business logic

### Requirement: Service Layer Responsibilities
Services SHALL contain all business logic, transaction management, and coordinate between repositories.

#### Scenario: Service manages transaction
- **WHEN** service method requires multiple repository calls
- **THEN** service wraps them in a database transaction

### Requirement: Repository Layer Responsibilities
Repositories SHALL only handle database operations: CRUD queries, row scanning, and SQL generation.

#### Scenario: Repository returns domain models
- **WHEN** repository fetches data from database
- **THEN** repository maps rows to domain model structs

### Requirement: Domain Models
Domain models SHALL be defined in `models.go` within each module and contain no database tags.

#### Scenario: Domain model structure
- **WHEN** domain model is defined
- **THEN** it uses `json:` tags for serialization
- **AND** database mapping happens in repository layer

### Requirement: Cross-Module Communication
Modules SHALL communicate only through HTTP API endpoints, not direct package imports.

#### Scenario: Forbidden import
- **WHEN** code in `internal/device` tries to import `internal/cicd`
- **THEN** Go compiler rejects due to module boundary violation

### Requirement: Shared Package Usage
Shared utilities SHALL be placed in `pkg/` and may be imported by any module.

#### Scenario: Config package usage
- **WHEN** any module needs configuration
- **THEN** module imports `pkg/config`

### Requirement: No Circular Dependencies
The dependency graph SHALL be acyclic with `cmd/` at the root.

#### Scenario: Dependency direction
- **WHEN** dependency direction is checked
- **THEN** cmd → internal/* → pkg (no reverse)
- **AND** internal/* does not depend on other internal/* packages
