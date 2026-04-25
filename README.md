# DevOps Toolkit

Go-based internal platform for managing infrastructure, CI/CD pipelines, logs, alerts, and physical hosts.

## Quick Start

```bash
# Clone and build
git clone https://github.com/devops-toolkit/devops-toolkit.git
cd devops-toolkit
go build -o devops-toolkit ./cmd/devops-toolkit

# Configure (optional - defaults work for local dev)
cp config.yaml config.local.yaml
# Edit config.local.yaml as needed

# Start the server
./devops-toolkit

# Verify it's running
curl http://localhost:3000/health
# Expected: {"status":"healthy"}
```

## Features

- **Devices** - Device state machine with PostgreSQL persistence
- **Pipelines** - CI/CD orchestration with stage runner
- **Logs** - Multi-backend log management (local/Elasticsearch/Loki)
- **Metrics** - Prometheus collector with `/metrics` endpoint
- **Alerts** - Notification channels with rate limiting
- **Kubernetes** - Cluster management (k3d/kind for testing, standard k8s for production)
- **Physical Hosts** - SSH monitoring and metrics collection
- **Projects** - Organizational hierarchy (Business Line → System → Project) with FinOps reporting
- **WebSocket** - Real-time event pub/sub

## Configuration

Configuration is via `config.yaml` with environment variable overrides.

```yaml
server:
  host: "0.0.0.0"
  port: 3000

database:
  host: "localhost"
  port: 5432
  user: "devops"
  password: "devops"
  name: "devops"

ldap:
  host: "ldap.example.com"
  port: 389
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_password: "admin"
  super_admin_group: "cn=SRE_Lead,ou=Groups,dc=example,dc=com"
```

Environment variable overrides: `DEVOPS_DATABASE_HOST`, `DEVOPS_LDAP_HOST`, etc.

## Development

```bash
# Run tests (unit + integration with real k3d clusters)
go test ./...

# Run tests only for a specific package
go test ./internal/k8s/...

# Start with hot-reload (requires fresh)
go run ./cmd/devops-toolkit
```

### K8s Integration Tests

Integration tests connect to real k3d clusters. They skip automatically if no cluster is configured.

```bash
# Set up a test cluster
k3d cluster create dev-cluster-1

# Tests will auto-detect and run against it
go test ./internal/k8s/... -v
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /api/devices` | List devices |
| `GET /api/pipelines` | List pipelines |
| `GET /api/logs` | Query logs |
| `GET /api/k8s/clusters` | List K8s clusters |
| `GET /api/org/business-lines` | List business lines |
| `WS /ws` | WebSocket for real-time events |

All API paths are relative (e.g., `api/k8s/clusters`) to support reverse proxy deployment.

## Project Structure

```
.
├── cmd/devops-toolkit/     # Main HTTP server
├── internal/
│   ├── config/             # YAML + env config loader
│   ├── device/             # Device state machine
│   ├── pipeline/           # CI/CD orchestration
│   ├── logs/               # Log manager (local/ES/Loki)
│   ├── metrics/            # Prometheus collector
│   ├── alerts/             # Notification channels
│   ├── k8s/                # Kubernetes cluster management
│   ├── physicalhost/       # SSH host monitoring
│   ├── project/            # Organizational hierarchy
│   └── websocket/          # Real-time events
├── scripts/                # Utility scripts
└── config.yaml             # Configuration
```

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - Backend architecture, data models, API structure
- [DESIGN.md](DESIGN.md) - Frontend design system
- [PRD.md](PRD.md) - Product requirements

## Deployment

See [ARCHITECTURE.md](ARCHITECTURE.md#deployment) for deployment options including systemd, Docker, and Kubernetes.
