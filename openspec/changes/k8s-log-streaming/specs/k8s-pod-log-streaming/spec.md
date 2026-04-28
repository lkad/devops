# k8s-pod-log-streaming

## ADDED Requirements

### Requirement: Real-time Log Streaming
The system SHALL stream Pod logs in real-time via WebSocket.

#### Scenario: Subscribe to pod log stream
- **WHEN** client sends WebSocket message to /ws with action "subscribe" and channel "container_log"
- **AND** payload contains cluster_id, namespace, pod, container
- **THEN** system starts streaming logs from K8s apiserver
- **AND** logs are broadcast to all subscribers of the channel

#### Scenario: Stream log message format
- **WHEN** new log line is received from K8s
- **THEN** message is broadcast in format:
```json
{
    "channel": "container_log",
    "type": "container_log",
    "data": {
        "clusterId": "uuid",
        "clusterName": "prod-cluster",
        "namespace": "default",
        "pod": "nginx-abc123",
        "container": "nginx",
        "message": "2026-04-27 10:00:00 GET /health 200",
        "level": "info",
        "timestamp": "2026-04-27T10:00:00Z"
    },
    "timestamp": "2026-04-27T10:00:00Z"
}
```

### Requirement: Log Persistence via Existing Logs Service
The system SHALL persist streamed logs through the existing logs service.

#### Scenario: Persist log entry
- **WHEN** new log line is received from K8s stream
- **THEN** system calls logsService.CreateLogEntry()
- **AND** log is stored in log_entries table with source="container"
- **AND** metadata contains cluster_id, pod, namespace, container

### Requirement: Historical Log Navigation
The system SHALL provide link to existing log management platform for historical logs.

#### Scenario: Get historical log link
- **WHEN** user requests GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/link
- **THEN** system returns URL to log management platform with pre-filled filters

```json
{
    "url": "/logs?source=container&cluster_id={clusterId}&namespace={namespace}&pod={pod}",
    "displayName": "View in Log Management"
}
```

### Requirement: Log Level Inference
The system SHALL infer log level from message content.

#### Scenario: Error level detection
- **WHEN** log message contains "error", "Error", "ERROR", "failed", "FATAL"
- **THEN** log level is set to "error"

#### Scenario: Warn level detection
- **WHEN** log message contains "warn", "Warning", "WARN"
- **THEN** log level is set to "warn"

#### Scenario: Default level
- **WHEN** log message does not match known patterns
- **THEN** log level is set to "info"

### Requirement: Multi-Container Support
The system SHALL support pods with multiple containers.

#### Scenario: Stream specific container
- **WHEN** client subscribes to pod log with container param
- **THEN** system streams logs only from specified container

#### Scenario: Stream all containers
- **WHEN** client subscribes to pod log without container param
- **THEN** system streams logs from all containers in the pod

### Requirement: Log Stream Disconnect Handling
The system SHALL handle client disconnect gracefully.

#### Scenario: Client disconnects
- **WHEN** WebSocket client disconnects
- **THEN** system stops K8s log watch for that subscription
- **AND** resources are cleaned up

### Requirement: Reconnect with Timestamp
The system SHALL support reconnection from a specific timestamp.

#### Scenario: Reconnect with since param
- **WHEN** client reconnects to log stream with since param
- **THEN** system retrieves logs from specified timestamp via logsService
- **AND** continues real-time streaming from that point

### Requirement: Log Source Identification
All K8s container logs SHALL be identifiable by source="container" in log queries.

#### Scenario: Query container logs
- **WHEN** user queries GET /api/logs?source=container
- **THEN** system returns all K8s container logs
- **AND** includes cluster_id, pod, namespace, container in metadata
