# log-aggregation

## Purpose

Define the log aggregation subsystem: a vendor-neutral API that works with multiple storage backends (Local, Elasticsearch, Loki) with capability-based graceful degradation. Clients can adapt to backend capabilities via a `/capabilities` endpoint, and the API never breaks when the backend is switched. All log operations are abstracted behind a `LogBackend` interface so new backends (ClickHouse, S3, etc.) can be added without API or client changes.

## Requirements

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

### Requirement: Backend Abstraction Interface
The system SHALL define a `LogBackend` interface that decouples the API layer from concrete storage implementations.

#### Scenario: Interface contract
- **WHEN** any backend is initialized
- **THEN** it implements `Query(ctx, LogQuery) (LogPage, error)`, `Stats(ctx, LogQuery) (Stats, error)`, `Capabilities() Capabilities`, `Health(ctx) error`

#### Scenario: Add new backend without API change
- **WHEN** a new backend (e.g., ClickHouse) is added
- **THEN** it implements the LogBackend interface
- **AND** no changes are required to the API layer or client code

#### Scenario: Backend selection at startup
- **WHEN** the server starts
- **THEN** it reads LOG_STORAGE_BACKEND env var
- **AND** instantiates exactly one backend (local, elasticsearch, or loki)
- **AND** registers it as the singleton LogBackend for the API layer

### Requirement: Backend Capabilities Declaration
The system SHALL expose backend capabilities via GET /api/logs/capabilities so clients can adapt their queries.

#### Scenario: Capabilities response
- **WHEN** client requests GET /api/logs/capabilities
- **THEN** system returns current backend name, version, and full Capabilities struct
- **AND** Capabilities includes: full_text_search, regex_search, structured_query, aggregations[], live_tail, max_time_range, max_page_size, queryable_fields[]

#### Scenario: Capabilities-driven UI
- **WHEN** frontend loads the log page
- **THEN** it first calls /api/logs/capabilities
- **AND** shows only the query controls that the backend supports
- **AND** disables controls for unsupported features (e.g., regex input hidden if backend has regex_search=false)

#### Scenario: Capabilities cache
- **WHEN** frontend makes multiple queries within 5 minutes
- **THEN** capabilities response is cached in memory
- **AND** a manual refresh button forces refetch

### Requirement: Universal Query DSL
The system SHALL accept a vendor-neutral `LogQuery` struct that all backends must support in its universal subset.

#### Scenario: Universal fields
- **WHEN** client sends a LogQuery
- **THEN** the following fields are accepted on every backend: start_time, end_time, level[], source[], search (substring), limit, offset, order_by
- **AND** all backends return identical JSON shape for LogEntry

#### Scenario: Advanced fields
- **WHEN** client sends advanced fields (regex, fields{}, structured_query)
- **THEN** API layer checks current backend capabilities
- **AND** if supported: passes through
- **AND** if not supported: returns 400 UNSUPPORTED_FEATURE with details and capabilities URL

#### Scenario: Default time range
- **WHEN** client sends no time range
- **THEN** API layer defaults to last 24 hours
- **AND** response meta includes `applied_defaults: {time_range: "24h"}`

#### Scenario: Limit bounds
- **WHEN** client requests limit > backend max_page_size
- **THEN** API caps limit to max_page_size
- **AND** response meta.degraded_features includes "limit: capped to N"
- **AND** pagination.has_more is set correctly

### Requirement: Graceful Degradation
The system SHALL degrade gracefully when a backend cannot fully satisfy a query, instead of failing.

#### Scenario: Time range exceeds backend max
- **WHEN** client requests 60 days and backend max_time_range is 720h (30 days)
- **THEN** system narrows the range to 30 days
- **AND** response meta.degraded_features includes "time_range: capped to 720h"
- **AND** response meta.applied_time_range shows actual range used

#### Scenario: Aggregation not supported
- **WHEN** client requests percentile aggregation on a backend that doesn't support it
- **THEN** system returns 400 UNSUPPORTED_FEATURE
- **AND** error.details.alternative suggests the closest supported aggregation

#### Scenario: Substring search fallback
- **WHEN** backend is Local and client sends search="error"
- **THEN** backend does case-insensitive substring match on message field
- **AND** meta.query_translated=false (native substring)

#### Scenario: Lucene query on Local backend
- **WHEN** backend is Local and client sends structured_query="level:error AND host:web-*"
- **THEN** system returns 400 UNSUPPORTED_FEATURE
- **AND** error.hint: "Local backend only supports search (substring). Use search field or switch to elasticsearch backend"

#### Scenario: Translation strategy reporting
- **WHEN** query is translated to backend-native form
- **THEN** response meta.translation_strategy is one of: "native" (no translation), "fallback" (degraded), "client-side" (filter applied after fetch)
- **AND** meta.degraded_features lists each degradation

### Requirement: Standard Error Codes
The system SHALL return standardized error codes that are independent of the active backend.

#### Scenario: Error code mapping
- **WHEN** backend returns a backend-specific error
- **THEN** API layer maps it to a standard ErrorCode
- **AND** response includes error, message, details, hint, docs_url

#### Scenario: Standard error codes
- **WHEN** any error occurs
- **THEN** response uses one of: INVALID_QUERY (400), UNSUPPORTED_FEATURE (400), QUERY_TIMEOUT (408), RESULT_TOO_LARGE (413), BACKEND_UNAVAILABLE (503), TIME_RANGE_EXCEEDED (400), RATE_LIMITED (429)

#### Scenario: Error response shape
- **WHEN** any error occurs
- **THEN** response is: `{"error": "CODE", "message": "...", "details": {...}, "hint": "...", "docs_url": "..."}`
- **AND** details contains structured info (e.g., {"max": "720h", "requested": "2160h"})
- **AND** hint suggests action (e.g., "Use /api/logs/capabilities")

#### Scenario: Backend unavailable
- **WHEN** Elasticsearch or Loki is unreachable
- **THEN** API returns 503 BACKEND_UNAVAILABLE
- **AND** details.backend_name indicates which backend
- **AND** Retry-After header is set to 30s

### Requirement: API Versioning
The system SHALL version the log API to allow safe evolution.

#### Scenario: Versioned endpoint
- **WHEN** client requests GET /api/v1/logs
- **THEN** system processes using v1 contract (current behavior)
- **AND** v1 remains stable as long as it is the current major version

#### Scenario: Backwards-compatible additions
- **WHEN** a new optional query parameter is added (e.g., `format=ndjson`)
- **THEN** it does not bump the major version
- **AND** old clients ignore it without error

#### Scenario: Breaking change
- **WHEN** a breaking change is needed (e.g., removing a parameter)
- **THEN** new version is created at /api/v2/logs
- **AND** /api/v1/logs continues to work for at least 6 months
- **AND** /api/v1/logs responses include `Deprecation` and `Sunset` HTTP headers

#### Scenario: Response schema stability
- **WHEN** client parses response.data
- **THEN** LogEntry schema is frozen for the major version
- **AND** new fields may be added but never removed
- **AND** new fields are added without breaking old parsers

### Requirement: Response Meta Envelope
The system SHALL include a `meta` object in every successful log response.

#### Scenario: Meta fields
- **WHEN** any log query succeeds
- **THEN** response includes `meta` with: backend (name), query_translated (bool), translation_strategy ("native"|"fallback"|"client-side"), degraded_features ([]), applied_defaults ({}), capabilities (subset)

#### Scenario: No silent behavior changes
- **WHEN** a backend change causes a query to behave differently
- **THEN** the change is reflected in meta fields
- **AND** clients can detect changes by diffing meta between requests
- **AND** behavior is never changed without meta notification

### Requirement: Compatibility Invariants
The system SHALL preserve the following invariants to guarantee backend-switch compatibility.

#### Scenario: LogEntry schema frozen
- **WHEN** backend is switched from Local to Elasticsearch to Loki
- **THEN** LogEntry JSON fields and types remain identical
- **AND** field order may differ but never breaks JSON parsing

#### Scenario: Universal subset always works
- **WHEN** client uses only universal fields (no advanced)
- **THEN** all three backends return semantically equivalent results
- **AND** no backend switch breaks the contract

#### Scenario: Capabilities query never fails
- **WHEN** backend is unavailable
- **THEN** /api/logs/capabilities still returns 200
- **AND** capabilities.backend is "unavailable"
- **AND** capabilities.supported_features is empty
- **AND** error in response body explains unavailability
