# api-contract

## ADDED Requirements

### Requirement: RESTful URL Structure
All API endpoints SHALL follow RESTful conventions with consistent URL structure.

#### Scenario: Resource URLs
- **WHEN** API endpoint represents a resource
- **THEN** URL is `/api/{resource}` (plural)
- **AND** Examples: `/api/devices`, `/api/pipelines`, `/api/alerts/channels`

#### Scenario: Nested resource URLs
- **WHEN** API endpoint represents a nested resource
- **THEN** URL is `/api/{parent}/:{parent_id}/{child}`
- **AND** Examples: `/api/business-lines/:id/systems`, `/api/systems/:id/projects`

### Requirement: Standard HTTP Methods
API SHALL use standard HTTP methods consistently.

| Method | Usage | Idempotent |
|--------|-------|------------|
| GET | Retrieve resource(s) | Yes |
| POST | Create resource | No |
| PUT | Replace resource | Yes |
| PATCH | Update resource fields | No |
| DELETE | Delete resource | Yes |

#### Scenario: GET list
- **WHEN** client sends GET /api/devices
- **THEN** server returns 200 with array of devices

#### Scenario: GET single
- **WHEN** client sends GET /api/devices/:id
- **THEN** server returns 200 with single device
- **OR** server returns 404 if not found

#### Scenario: POST create
- **WHEN** client sends POST /api/devices with body
- **THEN** server returns 201 with created device
- **AND** Location header with resource URL

#### Scenario: DELETE
- **WHEN** client sends DELETE /api/devices/:id
- **THEN** server returns 204 No Content on success

### Requirement: Standard List Response Format
List endpoints SHALL return consistent pagination envelope.

```json
{
    "data": [...],
    "pagination": {
        "total": 100,
        "limit": 20,
        "offset": 0,
        "has_more": true
    }
}
```

#### Scenario: Paginated list
- **WHEN** client requests GET /api/devices?limit=20&offset=0
- **THEN** response contains data array and pagination object

#### Scenario: List with filtering
- **WHEN** client requests GET /api/devices?state=active&type=physical_host
- **THEN** response contains only matching devices

### Requirement: Standard Error Response Format
All errors SHALL return consistent JSON format.

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Human readable message",
        "details": {}
    }
}
```

#### Scenario: Validation error
- **WHEN** request contains invalid data
- **THEN** server returns 400 with error.code="VALIDATION_ERROR"
- **AND** error.details contains field-level errors

#### Scenario: Not found error
- **WHEN** requested resource does not exist
- **THEN** server returns 404 with error.code="NOT_FOUND"

#### Scenario: Unauthorized error
- **WHEN** request has missing or invalid auth token
- **THEN** server returns 401 with error.code="UNAUTHORIZED"

#### Scenario: Forbidden error
- **WHEN** user lacks permission for operation
- **THEN** server returns 403 with error.code="FORBIDDEN"

### Requirement: Error Codes
The system SHALL define and use standard error codes.

| Code | HTTP Status | Description |
|------|-------------|-------------|
| VALIDATION_ERROR | 400 | Invalid request data |
| UNAUTHORIZED | 401 | Missing or invalid auth |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource not found |
| CONFLICT | 409 | Resource conflict (e.g., duplicate) |
| INVALID_STATE | 422 | Invalid state transition |
| RATE_LIMITED | 429 | Rate limit exceeded |
| INTERNAL_ERROR | 500 | Unexpected server error |

### Requirement: Content Type
All API responses SHALL use `application/json` content type.

#### Scenario: Response content type
- **WHEN** server sends API response
- **THEN** Content-Type header is "application/json"

### Requirement: Request Body Format
Request bodies SHALL be JSON with Content-Type header required.

#### Scenario: JSON body parsing
- **WHEN** POST request has Content-Type: application/json
- **THEN** body is parsed as JSON

#### Scenario: Missing content type
- **WHEN** POST request has no Content-Type
- **THEN** server returns 415 Unsupported Media Type
