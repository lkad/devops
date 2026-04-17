# LDAP Authentication Client

Go-based LDAP client for authenticating users and resolving their system roles.

## Features

- **Authentication**: Bind with user DN and password
- **Group Retrieval**: Search for group memberships
- **Role Mapping**: Map LDAP groups to internal system roles
- **Connection Pooling**: Efficient connection reuse
- **Rate Limiting**: Token bucket rate limiter
- **Retry Logic**: Exponential backoff on failures
- **Audit Logging**: Complete audit trail of auth events

## Group to Role Mapping

| LDAP Group | System Role |
|------------|-------------|
| `cn=IT_Ops,ou=Groups,dc=example,dc=com` | Operator |
| `cn=DevTeam_Payments,ou=Groups,dc=example,dc=com` | Developer |
| `cn=Security_Auditors,ou=Groups,dc=example,dc=com` | Auditor |
| `cn=SRE_Lead,ou=Groups,dc=example,dc=com` | SuperAdmin |

## Quick Start

### Prerequisites

- Go 1.21+
- LDAP server (or use Docker Compose for local dev)

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_URL` | `ldap://localhost:389` | LDAP server URL |
| `LDAP_BASE_DN` | `dc=example,dc=com` | Base DN for searches |
| `LDAP_ADMIN_DN` | `cn=admin,dc=example,dc=com` | Admin DN for binding |
| `LDAP_ADMIN_PASSWORD` | (empty) | Admin password |
| `LDAP_USER_SEARCH_BASE` | `ou=Users` | User search base |
| `LDAP_GROUP_SEARCH_BASE` | `ou=Groups` | Group search base |
| `LDAP_POOL_SIZE` | `10` | Connection pool size |
| `LDAP_CONNECT_TIMEOUT` | `10s` | Connection timeout |
| `LDAP_SEARCH_TIMEOUT` | `30s` | Search timeout |
| `LDAP_MAX_RETRIES` | `3` | Max retry attempts |
| `LDAP_RATE_LIMIT_PER_SECOND` | `100` | Rate limit |

### Usage

```go
package main

import (
    "fmt"
    "github.com/example/devops/auth/ldap"
)

func main() {
    // Create client with default config
    config := ldap.DefaultConfig()
    config.URL = "ldap://localhost:389"
    config.AdminPassword = "adminpassword"

    client, err := ldap.NewClient(config)
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Authenticate
    ok, err := client.Authenticate("cn=john,ou=Users,dc=example,dc=com", "password123")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Authenticated: %v\n", ok)

    // Get roles
    roles, err := client.GetRoles("cn=john,ou=Users,dc=example,dc=com")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Roles: %v\n", roles)
}
```

### Demo CLI

```bash
# Set environment variables
export TEST_LDAP_USER_DN="cn=john,ou=Users,dc=example,dc=com"
export TEST_LDAP_PASSWORD="password123"
export LDAP_ADMIN_PASSWORD="adminpassword"

# Run demo
go run cmd/testldap/main.go
```

## Testing

### Unit Tests

```bash
go test -v ./...
```

### Integration Tests

Start the test LDAP server:

```bash
docker compose up -d
```

Run integration tests:

```bash
RUN_INTEGRATION_TESTS=true go test -v ./...
```

### Performance Tests

```bash
go test -run TestAuthPerformance -bench=. ./...
```

### Coverage

```bash
go test -cover ./...
```

## Docker Compose for Local Development

```bash
# Start LDAP server
docker compose up -d

# Verify it's running
docker compose ps

# View logs
docker compose logs -f ldap

# Stop
docker compose down
```

The LDAP server will be available at `ldap://localhost:389` with:
- Admin DN: `cn=admin,dc=example,dc=com`
- Admin Password: `adminpassword`

## Project Structure

```
auth/ldap/
├── audit.go         # Audit logging
├── client.go       # Main LDAP client
├── config.go       # Configuration
├── pool.go         # Connection pooling
├── ratelimit.go    # Rate limiting
├── client_test.go  # Unit tests
└── go.mod          # Go module

cmd/testldap/
├── main.go         # Demo CLI
└── go.mod          # CLI module
```

## Error Handling

The client returns structured errors:

```go
// Authentication errors
&ldap.AuthError{
    UserDN: "cn=user,dc=example,dc=com",
    Reason: "invalid credentials",
}

// Configuration errors
&ldap.ConfigError{
    Field: "URL",
    Message: "LDAP_URL is required",
}
```

## Security Considerations

- Never log raw passwords
- Store credentials in sealed secrets for production
- Use LDAPS in production environments
- Rate limiting prevents brute force attacks
- Connection pooling reduces connection overhead

## License

MIT
