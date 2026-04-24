# Device Auto-Registration Guide

**Product:** DevOps Toolkit
**Version:** 1.3
**Last Updated:** 2026-04-19

---

## Overview

DevOps Toolkit automatically discovers and registers devices from your test environment. Instead of manually adding each device, the system scans your network, finds devices, and guides you through a simple approval workflow.

## How It Works

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Scan       │ ──▶ │  Review     │ ──▶ │  Register   │ ──▶ │  Manage      │
│  Network    │     │  Devices    │     │  Approved   │     │  Active      │
│             │     │             │     │  Devices    │     │  Devices     │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
     │                   │                   │                   │
     ▼                   ▼                   ▼                   ▼
  System           User reviews         User clicks        Full device
  probes           pending list         "Register"         control panel
  172.30.30.0/24
```

---

## Step-by-Step Instructions

### Prerequisites

- DevOps Toolkit running on `http://localhost:3000`
- Containerlab test environment deployed with 8 devices
- Network connectivity to `172.30.30.0/24`

### Step 1: Deploy Your Test Environment

If you have not already deployed the Containerlab topology:

```bash
cd /mnt/devops/devops-toolkit/test-environment/clab

# Install Containerlab (first time only)
sudo ./install.sh

# Deploy the dual-datacenter topology
sudo ./clab.sh deploy

# Verify all 8 nodes are running
sudo clab inspect -t topology.yml
```

Expected output shows 8 containers running:
- DC1: dc1-sw1, dc1-sw2, dc1-web, dc1-db
- DC2: dc2-sw1, dc2-sw2, dc2-web, dc2-db

### Step 2: Run Network Discovery

#### Option A: Using the Web UI

1. Open DevOps Toolkit at `http://localhost:3000`
2. Navigate to **Devices** → **Discovery**
3. Click **Scan Network**

#### Option B: Using the API

```bash
# Trigger a network scan
curl -X POST http://localhost:3000/api/discovery/scan

# Response
{
  "success": true,
  "devices": [
    {
      "id": "clab-dc1-web-21",
      "type": "physical_host",
      "name": "dc1-web",
      "labels": [
        { "datacenter": "dc1", "role": "server", "tier": "web" },
        { "protocol": "SSH" }
      ],
      "metadata": {
        "ip": "172.30.30.21",
        "port": 22,
        "discovered_at": "2026-04-19T10:30:00.000Z",
        "discovery_method": "ssh_scan"
      }
    },
    ...
  ]
}
```

#### Option C: Using the Test Script

```bash
cd /mnt/devops/devops-toolkit
node scripts/discovery-test.js
```

This runs a faster scan optimized for the test environment.

---

### Step 3: Review Discovered Devices

After scanning, view all discovered devices:

```bash
curl http://localhost:3000/api/discovery/status
```

**Discovered Devices:**

| Device ID | Name | Type | Datacenter | IP | Protocol |
|-----------|------|------|------------|-----|----------|
| clab-dc1-web-21 | dc1-web | physical_host | DC1 | 172.30.30.21 | SSH |
| clab-dc1-db-22 | dc1-db | physical_host | DC1 | 172.30.30.22 | SSH |
| clab-dc1-sw-core-11 | dc1-sw1 | network_device | DC1 | 172.30.30.11 | SNMP |
| clab-dc1-sw-dist-12 | dc1-sw2 | network_device | DC1 | 172.30.30.12 | SNMP |
| clab-dc2-web-41 | dc2-web | physical_host | DC2 | 172.30.30.41 | SSH |
| clab-dc2-db-42 | dc2-db | physical_host | DC2 | 172.30.30.42 | SSH |
| clab-dc2-sw-core-31 | dc2-sw1 | network_device | DC2 | 172.30.30.31 | SNMP |
| clab-dc2-sw-dist-32 | dc2-sw2 | network_device | DC2 | 172.30.30.32 | SNMP |

---

### Step 4: Register Devices

Register all discovered devices with a single command:

```bash
curl -X POST http://localhost:3000/api/discovery/register
```

**Response:**

```json
{
  "success": true,
  "registered": 8,
  "failed": 0,
  "devices": [
    {
      "id": "clab-dc1-web-21",
      "name": "dc1-web",
      "type": "physical_host",
      "status": "pending",
      ...
    },
    ...
  ],
  "errors": []
}
```

---

### Step 5: Approve Devices (Admin Only)

After registration, devices appear in **Pending** status. An administrator must approve them:

#### Using the API

```bash
# List all pending devices
curl http://localhost:3000/api/devices | jq '.devices[] | select(.status == "pending")'

# Authenticate a pending device (transition to AUTHENTICATED)
curl -X POST http://localhost:3000/api/devices/clab-dc1-web-21/authenticate \
  -H "Content-Type: application/json" \
  -d '{"token": "your-bootstrap-token"}'

# Complete registration (transition to REGISTERED)
curl -X POST http://localhost:3000/api/devices/clab-dc1-web-21/register \
  -H "Content-Type: application/json" \
  -d '{"labels": [{"env": "test"}]}'

# Activate device (transition to ACTIVE)
curl -X POST http://localhost:3000/api/devices/clab-dc1-web-21/activate
```

#### Using the Web UI

1. Open `http://localhost:3000`
2. Go to **Devices** → **Pending Approvals**
3. Click **Approve** next to each device
4. Devices move to **Active** status

---

### Step 6: View Active Devices

```bash
# List all active devices
curl http://localhost:3000/api/devices | jq '.devices[] | select(.status == "active")'
```

You now have full control over all 8 devices.

---

## Device State Machine

Devices progress through these states:

```
PENDING ──▶ AUTHENTICATED ──▶ REGISTERED ──▶ ACTIVE
                                      │
                                      ▼
                              MAINTENANCE ◀──▶ SUSPENDED
```

| State | Description |
|-------|-------------|
| **PENDING** | Device discovered but not yet approved |
| **AUTHENTICATED** | Device credentials verified |
| **REGISTERED** | Device configured and ready |
| **ACTIVE** | Device fully operational |
| **MAINTENANCE** | Device taken offline for maintenance |
| **SUSPENDED** | Device temporarily disabled |

---

## Managing Active Devices

### Execute Commands

```bash
# Restart a device
curl -X POST http://localhost:3000/api/devices/clab-dc1-web-21/actions \
  -H "Content-Type: application/json" \
  -d '{"action": "restart"}'
```

### View Device Metrics

```bash
curl http://localhost:3000/api/devices/clab-dc1-web-21/metrics
```

### View Device Events

```bash
curl http://localhost:3000/api/devices/clab-dc1-web-21/events
```

---

## Network Discovery API Reference

### Scan Network

```http
POST /api/discovery/scan
```

Scans the configured network range (default: `172.30.30.0/24`) for devices.

**Response:**

```json
{
  "success": true,
  "devices": [
    {
      "id": "clab-dc1-web-21",
      "type": "physical_host",
      "name": "dc1-web",
      "labels": [{ "datacenter": "dc1", "role": "server" }],
      "metadata": { "ip": "172.30.30.21", "port": 22 }
    }
  ]
}
```

---

### Get Discovery Status

```http
GET /api/discovery/status
```

Returns the status of the last scan.

**Response:**

```json
{
  "success": true,
  "status": {
    "isScanning": false,
    "lastScan": {
      "timestamp": "2026-04-19T10:30:00.000Z",
      "switchesFound": 4,
      "serversFound": 4
    }
  }
}
```

---

### Register Discovered Devices

```http
POST /api/discovery/register
```

Creates pending device entries for all previously scanned devices.

**Response:**

```json
{
  "success": true,
  "registered": 8,
  "failed": 0,
  "devices": [...],
  "errors": []
}
```

---

## Troubleshooting

### Scan Finds No Devices

1. Verify Containerlab is running:
   ```bash
   sudo clab inspect -t topology.yml
   ```

2. Test connectivity from DevOps Toolkit host:
   ```bash
   ping 172.30.30.21
   docker exec clab-devops-dc-ha-dc1-web hostname
   ```

3. Check SSH is accessible:
   ```bash
   nc -zv 172.30.30.21 22
   ```

### Device Registration Fails

- Ensure the device ID is unique
- Check the device is not already registered
- Verify DevOps Toolkit has write access to device config directory

### Device Stays in PENDING

- Complete the authentication step before registration
- Use the correct bootstrap token

---

## Containerlab Test Environment Reference

### Deploy Topology

```bash
cd /mnt/devops/devops-toolkit/test-environment/clab
sudo ./clab.sh deploy
```

### Node Access Commands

| Node | Access Command |
|------|----------------|
| DC1 Web | `docker exec -it clab-devops-dc-ha-dc1-web bash` |
| DC1 DB | `docker exec -it clab-devops-dc-ha-dc1-db bash` |
| DC1 Switch SNMP | `docker exec clab-devops-dc-ha-dc1-sw1 snmpwalk -v 2c -c public localhost` |
| DC2 Web | `docker exec -it clab-devops-dc-ha-dc2-web bash` |
| DC2 DB | `docker exec -it clab-devops-dc-ha-dc2-db bash` |
| DC2 Switch SNMP | `docker exec clab-devops-dc-ha-dc2-sw1 snmpwalk -v 2c -c public localhost` |

### Destroy Topology

```bash
cd /mnt/devops/devops-toolkit/test-environment/clab
sudo ./clab.sh destroy
```

---

## Support

For issues or questions:
- Check DevOps Toolkit logs: `http://localhost:3000/health`
- Review Containerlab status: `sudo clab inspect -t topology.yml`
- Consult the full PRD documentation
