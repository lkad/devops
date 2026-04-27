# middleware-stack

## ADDED Requirements

### Requirement: Middleware Execution Order
Middleware SHALL be executed in specific order: CORS → Recovery → Logging → Metrics → Auth → RBAC.

#### Scenario: Middleware order
- **WHEN** HTTP request is received
- **THEN** middleware executes in defined order
- **AND** each middleware passes control to next via next()

### Requirement: CORS Middleware
CORS middleware SHALL set appropriate headers for cross-origin requests.

```go
func CORS() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
            // ...
        })
    }
}
```

#### Scenario: Preflight request
- **WHEN** OPTIONS request is received
- **THEN** CORS headers are set and 204 is returned

### Requirement: Recovery Middleware
Recovery middleware SHALL catch panics and return 500 error.

#### Scenario: Panic recovery
- **WHEN** handler panics
- **THEN** middleware recovers and logs stack trace
- **AND** returns 500 Internal Server Error to client

### Requirement: Logging Middleware
Logging middleware SHALL log all HTTP requests with method, path, status, and duration.

```go
type LogEntry struct {
    Method     string
    Path       string
    StatusCode int
    Duration   time.Duration
    ClientIP   string
}
```

#### Scenario: Request logging
- **WHEN** HTTP request completes
- **THEN** request is logged with method, path, status, duration

### Requirement: Metrics Middleware
Metrics middleware SHALL record HTTP request metrics.

#### Scenario: Record metrics
- **WHEN** HTTP request completes
- **THEN** http_requests_total counter is incremented
- **AND** http_request_duration_ms histogram is observed

### Requirement: Auth Middleware
Auth middleware SHALL validate JWT tokens on protected routes.

#### Scenario: Valid token
- **WHEN** request has valid Authorization: Bearer <token>
- **THEN** user info is extracted and added to context
- **AND** request passes to handler

#### Scenario: Missing token
- **WHEN** request to protected route has no token
- **THEN** 401 Unauthorized is returned

#### Scenario: Invalid token
- **WHEN** request has invalid or expired token
- **THEN** 401 Unauthorized is returned

### Requirement: Public Routes
Some routes SHALL be public and skip auth middleware.

#### Scenario: Public endpoints
- **WHEN** request is to /health, /metrics, or /api/auth/*
- **THEN** auth middleware is skipped

### Requirement: RBAC Middleware
RBAC middleware SHALL enforce permission checks based on user role and resource.

#### Scenario: Allowed operation
- **WHEN** user with operator role accesses deploy endpoint
- **THEN** request passes to handler

#### Scenario: Forbidden operation
- **WHEN** user with developer role accesses admin endpoint
- **THEN** 403 Forbidden is returned

### Requirement: Middleware Chaining
Middleware SHALL be chainable using adapter pattern.

```go
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}
```

### Requirement: Context Propagation
Middleware SHALL propagate request context to handlers.

#### Scenario: Context access
- **WHEN** handler accesses r.Context()
- **THEN** middleware values (user, request ID) are available
