# DevOps Toolkit - Agent Context

## Quick Start Commands

```
# Start agent
npm start

# Run tests (jest)
npm test

# Run device lab tests
npm run test:test-lab

# Docker test environment
npm run docker:up
npm run docker:down

# Full integration test
npm run test:full

# Dev mode
npm run dev
```

## Architecture Overview

**Entry Points**:
- `cmd/main.js` - CLI entry point for all commands
- `devices/agent.js` - DevToolsAgent class (core orchestration)
- `devices/device_manager.js` - DeviceManager class (device lifecycle)
- `config/config.js` - System configuration & device types

**Directory Ownership**:
- `/cmd/` - CLI commands (main.js orchestrates)
- `/devices/` - Agent & device manager code
- `/config/` - System config, device type templates
- `/tests/` - Unit tests (jest + custom lab scripts)
- `/devices/docker/` - Docker Compose test environment

## Key Constraints & Gotchas

### Bash Command Quirks
- **DO NOT** use `cd && docker ps` - Bash tool fails with `workdir` param only
- **DO USE** `cd /path && command` OR `command with workdir` parameter

### Package Scripts
- `npm test` runs jest with custom config
- `npm run test:test-lab` runs bash script in `devices/scripts/`
- `npm run test:full` is atomic: `docker up → run tests → docker down`

### Docker Environment
- `docker-compose.test.yml` defines 6 device tiers:
  - Container layer (nginx, node, mysql, redis)
  - Network layer (router, loadbalancer)
  - Monitoring layer (node-exporter, filebeat)
- **DO NOT** run `test:full` on production - it tears down after tests

### Device Types Configured
- `deviceTypes.container` → ['Docker', 'Kubernetes']
- `deviceTypes.physical_host` → ['SSH', 'SCP', 'WinRM']
- `deviceTypes.network_device` → ['SNMP', 'NETCONF']
- `deviceTypes.cloud_instance` → ['API']
- `deviceTypes.iot_device` → ['MQTT', 'HTTP']

### Testing Prerequisites
- Jest config at `test/jest.config.js` (required for `npm test`)
- Bash test script at `devices/scripts/run-tests.sh`
- Docker must be running for integration tests

## Testing Strategy

```
Unit Tests (Jest):    npm test
Lab Tests (Bash):     npm run test:test-lab
Integration (Docker): npm run test:full
```

## Common Pitfalls

1. **Bash command fails** → Use quoted paths: `"command with spaces"`
2. **Docker not found** → Verify `docker ps` works
3. **Tests fail silently** → Check `test:full` output carefully
4. **Agent not connecting** → Ensure port 3001 is free

## Documentation

- `docs/DESIGN.md` - System architecture (681 lines)
- `package.json` - All npm scripts defined
- Docker Compose at `devices/docker/docker-compose.test.yml`
