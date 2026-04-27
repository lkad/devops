# alert-notification

## ADDED Requirements

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
