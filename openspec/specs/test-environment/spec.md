# test-environment

## ADDED Requirements

### Requirement: Containerlab Topology
The system SHALL provide Containerlab-based dual-datacenter test topology.

#### Scenario: Deploy topology
- **WHEN** user runs clab.sh deploy
- **THEN** Containerlab creates dual-datacenter topology with 8 nodes

#### Scenario: Node inventory
- **WHEN** topology is deployed
- **THEN** system has 8 nodes: dc1-sw1, dc1-sw2, dc1-web, dc1-db, dc2-sw1, dc2-sw2, dc2-web, dc2-db

### Requirement: Dual Datacenter Architecture
The system SHALL simulate two independent datacenters with trunk links.

#### Scenario: Dual trunk links
- **WHEN** topology is deployed
- **THEN** two trunk connections exist between dc1-sw1 and dc2-sw1 (eth2, eth3)

#### Scenario: DC network isolation
- **WHEN** DC1 and DC2 are deployed
- **THEN** DC1 uses 10.0.1.0/24, DC2 uses 10.0.2.0/24

### Requirement: Simulated Device Types
The system SHALL simulate PhysicalHost, NetworkDevice, and Container.

#### Scenario: PhysicalHost simulation
- **WHEN** containerlab web node is deployed
- **THEN** SSH is available on port 22 with real openssh-server

#### Scenario: NetworkDevice simulation
- **WHEN** containerlab switch node is deployed
- **THEN** SNMP is available on UDP:161 with net-snmpd

#### Scenario: Container simulation
- **WHEN** containerlab container node is deployed
- **THEN** Docker API is accessible for native container operations

### Requirement: Connection Verification
The system SHALL provide scripts to verify connectivity.

#### Scenario: SSH verification
- **WHEN** user runs verify.sh with SSH target
- **THEN** script tests SSH connection and returns success/failure

#### Scenario: SNMP verification
- **WHEN** user runs verify.sh with SNMP target
- **THEN** script tests SNMP query and returns device info

### Requirement: Device Auto-Registration to DevOps Toolkit
The system SHALL automatically register discovered devices.

#### Scenario: Discovery flow
- **WHEN** containerlab topology is running and DevOps Toolkit is active
- **THEN** NetworkDiscovery scans 172.30.30.0/24 and creates PENDING devices

### Requirement: Simulated Metrics
The system SHALL generate realistic metrics for testing.

#### Scenario: Physical host metrics
- **WHEN** metrics are collected from physical host
- **THEN** system returns CPU, memory, disk, uptime data

#### Scenario: Network device metrics
- **WHEN** metrics are collected from network device via SNMP
- **THEN** system returns interface status, traffic octets, uptime

### Requirement: Time Series Database Integration
The system SHALL include InfluxDB and Prometheus for metrics storage.

#### Scenario: InfluxDB available
- **WHEN** docker-compose is started
- **THEN** InfluxDB is available on port 8086

#### Scenario: Prometheus available
- **WHEN** docker-compose is started
- **THEN** Prometheus is available on port 9090

#### Scenario: Grafana available
- **WHEN** docker-compose is started
- **THEN** Grafana is available on port 3001
