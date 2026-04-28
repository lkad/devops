# http-router-framework

## ADDED Requirements

### Requirement: Gin Framework Integration
The system SHALL use Gin as the HTTP routing framework, replacing gorilla/mux.

#### Scenario: Gin engine initialization
- **WHEN** server starts
- **THEN** Gin engine is created with `gin.Default()` or `gin.New()`

#### Scenario: Route registration
- **WHEN** API routes are registered
- **THEN** they use Gin methods (GET, POST, PUT, DELETE)

### Requirement: Route Compatibility
All existing API routes SHALL maintain identical paths after migration.

#### Scenario: Cluster routes
- **WHEN** client calls `GET /api/k8s/clusters`
- **THEN** same handler is invoked as before migration

#### Scenario: Path parameters
- **WHEN** client calls `GET /api/k8s/clusters/{name}/pods`
- **THEN** `c.Param("name")` returns the cluster name

### Requirement: Middleware Integration
Middleware SHALL be adapted to Gin middleware signature `func(*gin.Context)`.

#### Scenario: Auth middleware
- **WHEN** request has valid JWT token
- **THEN** user context is available via `c.Get("user")`

#### Scenario: Metrics middleware
- **WHEN** HTTP request completes
- **THEN** request metrics are recorded

### Requirement: WebSocket Compatibility
WebSocket connections SHALL continue to use gorilla/websocket through Gin route handler.

#### Scenario: WebSocket upgrade
- **WHEN** client connects to `/ws`
- **THEN** WebSocket upgrade succeeds and hub handles connection

### Requirement: Response Format
API responses SHALL maintain identical JSON format.

#### Scenario: JSON response
- **WHEN** handler returns data
- **THEN** `c.JSON()` produces same JSON as previous `json.NewEncoder(w).Encode()`

### Requirement: Graceful Shutdown
Server SHALL support graceful shutdown with context timeout.

#### Scenario: Shutdown signal
- **WHEN** SIGINT or SIGTERM is received
- **THEN** in-flight requests complete within timeout
- **AND** server exits cleanly
