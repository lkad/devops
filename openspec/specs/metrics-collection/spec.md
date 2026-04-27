# metrics-collection

## ADDED Requirements

### Requirement: Prometheus Metrics Endpoint
The system SHALL expose metrics in Prometheus text format at GET /metrics.

#### Scenario: Scrape metrics
- **WHEN** Prometheus scrapes GET /metrics
- **THEN** system returns metrics in Prometheus text format

### Requirement: JSON Metrics API
The system SHALL provide JSON-formatted metrics at GET /api/metrics.

#### Scenario: Get all metrics in JSON
- **WHEN** user requests GET /api/metrics
- **THEN** system returns all current metrics with values and labels

### Requirement: Counter Metrics
The system SHALL support counter metrics via POST /api/metrics/counter.

#### Scenario: Increment counter
- **WHEN** user sends POST /api/metrics/counter with name, value, and labels
- **THEN** system increments counter by specified value

### Requirement: Gauge Metrics
The system SHALL support gauge metrics via POST /api/metrics/gauge.

#### Scenario: Set gauge value
- **WHEN** user sends POST /api/metrics/gauge with name, value, and labels
- **THEN** system sets gauge to specified value

#### Scenario: Increment gauge
- **WHEN** user sends POST /api/metrics/gauge with operation=inc
- **THEN** system increments gauge by value

#### Scenario: Decrement gauge
- **WHEN** user sends POST /api/metrics/gauge with operation=dec
- **THEN** system decrements gauge by value

### Requirement: Histogram Metrics
The system SHALL support histogram metrics via POST /api/metrics/histogram.

#### Scenario: Observe histogram value
- **WHEN** user sends POST /api/metrics/histogram with name, value, and labels
- **THEN** system records observation in histogram

### Requirement: HTTP Request Metrics
The system SHALL automatically collect HTTP request metrics.

#### Scenario: Track request
- **WHEN** HTTP request completes
- **THEN** system increments http_requests_total with endpoint, method, status labels

#### Scenario: Track latency
- **WHEN** HTTP request completes
- **THEN** system observes request duration in http_request_duration_ms histogram

### Requirement: Module-Specific Metrics
The system SHALL collect metrics for device events, pipeline events, and alerts.

#### Scenario: Device event metric
- **WHEN** device event occurs
- **THEN** system increments device_events_total with event type

#### Scenario: Pipeline event metric
- **WHEN** pipeline event occurs
- **THEN** system increments pipeline_events_total with pipeline and event type

#### Scenario: Alert metric
- **WHEN** alert triggers
- **THEN** system increments alerts_total with alert name and severity
