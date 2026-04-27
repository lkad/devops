# log-aggregation

## ADDED Requirements

### Requirement: Storage Backend Selection
The system SHALL support Local, Elasticsearch, and Loki storage backends configurable via LOG_STORAGE_BACKEND environment variable.

#### Scenario: Local backend (default)
- **WHEN** LOG_STORAGE_BACKEND=local or not set
- **THEN** system uses LocalStorageBackend for all log operations

#### Scenario: Elasticsearch backend
- **WHEN** LOG_STORAGE_BACKEND=elasticsearch
- **THEN** system delegates all log operations to ElasticsearchBackend

#### Scenario: Loki backend
- **WHEN** LOG_STORAGE_BACKEND=loki
- **THEN** system delegates all log operations to LokiBackend

### Requirement: Log Query API
The system SHALL provide log query capabilities via GET /api/logs with filters.

#### Scenario: Query logs with level filter
- **WHEN** user requests GET /api/logs?level=error
- **THEN** system returns all error-level logs

#### Scenario: Query logs with time range
- **WHEN** user requests GET /api/logs?start=2026-04-01&end=2026-04-30
- **THEN** system returns logs within specified time range

#### Scenario: Query logs with search term
- **WHEN** user requests GET /api/logs?search=failed
- **THEN** system returns logs containing "failed" in message

### Requirement: Log Statistics
The system SHALL provide log statistics via GET /api/logs/stats.

#### Scenario: Get log statistics
- **WHEN** user requests GET /api/logs/stats
- **THEN** system returns counts by level, daily distribution, total count

### Requirement: Backend Health Check
The system SHALL expose backend health via GET /api/logs/backend.

#### Scenario: Backend healthy
- **WHEN** user requests GET /api/logs/backend
- **THEN** system returns backend type and health status

### Requirement: Retention Policy
The system SHALL support configurable log retention.

#### Scenario: Get retention policy
- **WHEN** user requests GET /api/logs/retention
- **THEN** system returns current retention configuration

#### Scenario: Update retention policy
- **WHEN** user sends PUT /api/logs/retention with new retention_days
- **THEN** system updates retention configuration

#### Scenario: Trigger retention cleanup
- **WHEN** user sends POST /api/logs/retention/apply
- **THEN** system executes retention cleanup and returns count of deleted logs

### Requirement: Alert Rules
The system SHALL support log-based alert rules via GET/POST /api/logs/alerts.

#### Scenario: Create alert rule
- **WHEN** user sends POST /api/logs/alerts with condition and notification_channel
- **THEN** system creates alert rule and returns rule ID

#### Scenario: List alert rules
- **WHEN** user requests GET /api/logs/alerts
- **THEN** system returns all configured alert rules

### Requirement: Saved Filters
The system SHALL support saved filter configurations via GET/POST /api/logs/filters.

#### Scenario: Create saved filter
- **WHEN** user sends POST /api/logs/filters with name and filter criteria
- **THEN** system saves filter and returns filter ID

#### Scenario: Apply saved filter
- **WHEN** user requests GET /api/logs/filters/:id
- **THEN** system returns logs matching saved filter criteria

### Requirement: Sample Log Generation
The system SHALL generate sample logs for testing via POST /api/logs/generate.

#### Scenario: Generate sample logs
- **WHEN** user sends POST /api/logs/generate with count and levels
- **THEN** system generates specified number of sample logs at specified levels
