# physical-host-monitoring

## ADDED Requirements

### Requirement: Host Registration
The system SHALL register physical hosts with SSH credentials.

#### Scenario: Register host
- **WHEN** user sends POST /api/physical-hosts with hostname, ip, port, credentials
- **THEN** system registers host and returns host ID with state=online

#### Scenario: Remove host
- **WHEN** user sends DELETE /api/physical-hosts/:id
- **THEN** system removes host and stops monitoring

### Requirement: SSH Connection Management
The system SHALL manage SSH connections with connection pooling.

#### Scenario: Pooled connections
- **WHEN** multiple requests target same host
- **THEN** system reuses SSH connections from pool

### Requirement: SSH Health Check
The system SHALL perform periodic SSH health checks.

#### Scenario: Successful health check
- **WHEN** SSH health check succeeds
- **THEN** system updates lastHeartbeat timestamp

#### Scenario: Failed health check
- **WHEN** SSH health check fails (connection refused, timeout)
- **THEN** system marks host state as offline if consecutive failures exceed threshold

### Requirement: Metrics Collection
The system SHALL collect CPU, memory, disk, and uptime metrics via SSH.

#### Scenario: Collect CPU metrics
- **WHEN** user requests GET /api/physical-hosts/:id/metrics
- **THEN** system executes SSH commands to collect CPU usage, cores

#### Scenario: Collect memory metrics
- **WHEN** user requests GET /api/physical-hosts/:id/metrics
- **THEN** system returns memory total, used, usagePercent

#### Scenario: Collect disk metrics
- **WHEN** user requests GET /api/physical-hosts/:id/metrics
- **THEN** system returns disk devices, sizes, used space, usagePercent

#### Scenario: Collect uptime
- **WHEN** user requests GET /api/physical-hosts/:id/metrics
- **THEN** system returns uptime value and formatted string

### Requirement: Service Monitoring
The system SHALL monitor system services via systemctl.

#### Scenario: List services
- **WHEN** user requests GET /api/physical-hosts/:id/services
- **THEN** system returns list of services with status (running, stopped)

### Requirement: Configuration Push
The system SHALL push configuration to hosts via SSH.

#### Scenario: Push config
- **WHEN** user sends POST /api/physical-hosts/:id/config with config content
- **THEN** system writes config to host via SSH

### Requirement: Two-Layer Architecture
The system SHALL separate Node Status Layer from Data Query Layer.

#### Scenario: Node status independent of DB
- **WHEN** InfluxDB is down but SSH check succeeds
- **THEN** host state remains online

#### Scenario: Data query with DB failure
- **WHEN** InfluxDB/Prometheus is unreachable
- **THEN** system returns cached data with dataStatus=stale

### Requirement: Local Cache
The system SHALL cache recent metrics for fast response and DB故障 fallback.

#### Scenario: Return cached data
- **WHEN** DB query times out
- **THEN** system returns cached data with dataStatus=stale

#### Scenario: Cache unavailable and DB down
- **WHEN** cache is empty and DB is unreachable
- **THEN** system returns dataStatus=unavailable

### Requirement: State Change Events
The system SHALL emit state change events when host state transitions occur.

#### Scenario: Emit state change event
- **WHEN** host state changes from online to offline
- **THEN** system broadcasts device_event to WebSocket subscribers
- **AND** event contains previous state, new state, and timestamp

### Requirement: WebSocket Status Updates
The system SHALL broadcast host status updates via WebSocket.

#### Scenario: Broadcast online status
- **WHEN** host health check succeeds
- **THEN** system broadcasts updated host status to device_event channel

#### Scenario: Broadcast offline status
- **WHEN** host is marked offline due to failed health checks
- **THEN** system broadcasts offline status to device_event channel