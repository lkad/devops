# network-discovery

## ADDED Requirements

### Requirement: Network Scanning
The system SHALL scan networks for device discovery via SNMP and SSH probing.

#### Scenario: Trigger network scan
- **WHEN** user sends POST /api/discovery/scan with network range
- **THEN** system scans network and reports discovered devices

#### Scenario: Discover SSH devices
- **WHEN** TCP port 22 is open on target host
- **THEN** system identifies device as physical_host

#### Scenario: Discover SNMP devices
- **WHEN** UDP port 161 responds to SNMP queries
- **THEN** system identifies device as network_device

### Requirement: Pull-Based Discovery
The system SHALL use pull-based discovery where DevOps Toolkit actively probes the network.

#### Scenario: Active probing
- **WHEN** network scan is triggered
- **THEN** DevOps Toolkit initiates SNMP/SSH probes to discovered devices

### Requirement: Device Auto-Registration
The system SHALL support automatic device registration workflow.

#### Scenario: Create pending devices
- **WHEN** scan discovers new devices
- **THEN** system creates devices in PENDING state

#### Scenario: Approve pending devices
- **WHEN** user sends POST /api/discovery/register with device IDs
- **THEN** system transitions devices from PENDING to AUTHENTICATED state

### Requirement: Discovery Status
The system SHALL report discovery scan status.

#### Scenario: Get scan status
- **WHEN** user requests GET /api/discovery/status
- **THEN** system returns last scan time, devices found, pending count

### Requirement: Device ID Naming
The system SHALL assign consistent device IDs based on discovery source.

#### Scenario: Containerlab device naming
- **WHEN** device discovered from containerlab topology
- **THEN** system assigns ID like clab-dc1-web-21

### Requirement: Discovery Manager
The system SHALL provide NetworkDiscovery class for scanning operations.

#### Scenario: Scan with timeout
- **WHEN** NetworkDiscovery.scan() is called with network and timeout
- **THEN** system probes each host with specified timeout

#### Scenario: Get discovered devices
- **WHEN** scan completes
- **THEN** NetworkDiscovery.getDiscoveredDevices() returns array of discovered devices
