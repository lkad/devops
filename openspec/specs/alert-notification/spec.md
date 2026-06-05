# alert-notification

## Purpose

Define the alert routing and delivery subsystem. Alerts are matched against rules, routed to channels (Slack, PagerDuty, webhook, email, log), and dispatched with deduplication and silencing. The subsystem respects maintenance mode: when a physical host is in maintenance, alerts about that host are suppressed for external channels but still recorded in the log channel for audit. The `SuppressedAlert` view exposes suppressed alerts to auditors with reason and maintenance window.

## Requirements

### Requirement: Notification Channels
The system SHALL support multiple notification channel types: slack, webhook, email, log.

#### Scenario: Create Slack channel
- **WHEN** user sends POST /api/alerts/channels with type=slack, webhookUrl, channel
- **THEN** system creates Slack notification channel

#### Scenario: Create Webhook channel
- **WHEN** user sends POST /api/alerts/channels with type=webhook, url, headers
- **THEN** system creates generic webhook channel

#### Scenario: Create Email channel
- **WHEN** user sends POST /api/alerts/channels with type=email, recipients
- **THEN** system creates email notification channel

#### Scenario: Create Log channel
- **WHEN** user sends POST /api/alerts/channels with type=log
- **THEN** system creates log-only notification channel

### Requirement: List Notification Channels
The system SHALL list all configured notification channels via GET /api/alerts/channels.

#### Scenario: List all channels
- **WHEN** user requests GET /api/alerts/channels
- **THEN** system returns array of all configured channels with type and configuration

### Requirement: Delete Notification Channel
The system SHALL remove notification channels via DELETE /api/alerts/channels/:name.

#### Scenario: Delete channel
- **WHEN** user sends DELETE /api/alerts/channels/:name
- **THEN** system removes channel and returns 204

### Requirement: Rate Limiting
The system SHALL enforce rate limiting of 10 alerts per 60 seconds per alert name.

#### Scenario: Within rate limit
- **WHEN** alert triggers with name=high_cpu and less than 10 alerts in last 60s
- **THEN** system sends notification immediately

#### Scenario: Rate limit exceeded
- **WHEN** alert triggers with name=high_cpu and 10+ alerts in last 60s
- **THEN** system queues alert and sends when rate window resets

### Requirement: Alert History
The system SHALL store and query alert history via GET /api/alerts/history.

#### Scenario: Query alert history
- **WHEN** user requests GET /api/alerts/history
- **THEN** system returns paginated list of triggered alerts with timestamps

#### Scenario: Filter by alert name
- **WHEN** user requests GET /api/alerts/history?name=high_cpu
- **THEN** system returns only alerts with name=high_cpu

### Requirement: Alert Statistics
The system SHALL provide alert statistics via GET /api/alerts/stats.

#### Scenario: Get alert stats
- **WHEN** user requests GET /api/alerts/stats
- **THEN** system returns total count, by severity, by name, last 24h distribution

### Requirement: Trigger Alert API
The system SHALL support programmatic alert triggering via POST /api/alerts/trigger.

#### Scenario: Trigger alert
- **WHEN** user sends POST /api/alerts/trigger with name, severity, message, channel
- **THEN** system triggers alert notification through specified channel

### Requirement: Maintenance Mode Alert Suppression
The system SHALL suppress external alert notifications when the target physical host is in maintenance mode.

#### Scenario: Suppress external notification
- **WHEN** an alert would be triggered and target is a physical host in maintenance mode
- **THEN** system does NOT send to external channels (slack, webhook, email)
- **AND** system writes alert_history entry with `suppressed=true`, `suppression_reason=host_in_maintenance`
- **AND** system broadcasts suppression event to WebSocket `alert` channel (internal only)

#### Scenario: Log channel still receives
- **WHEN** alert is suppressed due to maintenance and channel type=log
- **THEN** system writes to log (internal record) but does not flag as external
- **AND** log entry is tagged with `suppressed=true`

#### Scenario: Alert resumes after maintenance exit
- **WHEN** host exits maintenance mode
- **THEN** subsequent alerts for that host are sent normally
- **AND** alerts suppressed during maintenance are NOT replayed retroactively

#### Scenario: Non-physical-host alerts not affected
- **WHEN** alert target is not a physical host (e.g., pipeline, log source)
- **THEN** system sends notifications normally
- **AND** maintenance mode on a host does not affect unrelated alerts

### Requirement: Suppression Visibility
The system SHALL provide visibility into suppressed alerts via the alert history API.

#### Scenario: List suppressed alerts
- **WHEN** user requests GET /api/alerts/history?suppressed=true
- **THEN** system returns alerts that were suppressed due to maintenance
- **AND** each entry includes host_id, maintenance_started_at, and reason

#### Scenario: Alert stats include suppression count
- **WHEN** user requests GET /api/alerts/stats
- **THEN** system returns additional field `suppressed_count` (last 24h)
- **AND** `suppressed_by_host` breakdown
