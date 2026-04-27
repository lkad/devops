# audit-logging

## ADDED Requirements

### Requirement: Audit Log Entry Creation
The system SHALL create audit log entries for all CRUD operations on auditable entities.

#### Scenario: Log create operation
- **WHEN** user creates business_line, system, project, resource_link, or permission
- **THEN** system creates audit log entry with action=create

#### Scenario: Log update operation
- **WHEN** user updates business_line, system, or project
- **THEN** system creates audit log entry with action=update

#### Scenario: Log delete operation
- **WHEN** user deletes business_line, system, project, resource_link, or permission
- **THEN** system creates audit log entry with action=delete

### Requirement: Audit Log Fields
Each audit log entry SHALL contain: id, timestamp, username, action, entity_type, entity_id, entity_name, changes, old_value, new_value, ip_address.

#### Scenario: Audit log structure
- **WHEN** audit log is created
- **THEN** entry contains all required fields with appropriate types

### Requirement: Audit Log Query API
The system SHALL provide GET /api/org/audit-logs for querying audit logs.

#### Scenario: Filter by entity type
- **WHEN** user requests GET /api/org/audit-logs?entity_type=business_line
- **THEN** system returns audit logs for business_line entities only

#### Scenario: Filter by entity ID
- **WHEN** user requests GET /api/org/audit-logs?entity_id=abc123
- **THEN** system returns audit logs for entity abc123

#### Scenario: Filter by username
- **WHEN** user requests GET /api/org/audit-logs?username=john
- **THEN** system returns audit logs for user john

#### Scenario: Pagination
- **WHEN** user requests GET /api/org/audit-logs?limit=20&offset=40
- **THEN** system returns 20 audit logs starting at offset 40

### Requirement: Audit Log Storage
The system SHALL store audit logs in PostgreSQL.

#### Scenario: Audit log persistence
- **WHEN** audit event occurs
- **THEN** system stores audit log in audit_logs table

### Requirement: Audit Log Retention
The system SHALL retain audit logs per compliance requirements.

#### Scenario: Audit log retention
- **WHEN** audit logs exceed retention period
- **THEN** system archives or deletes old logs per policy
