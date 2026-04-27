# device-management

## ADDED Requirements

### Requirement: Device State Machine
The system SHALL enforce a strict device state machine with validated transitions: PENDING → AUTHENTICATED → REGISTERED → ACTIVE → MAINTENANCE/SUSPENDED → RETIRE.

#### Scenario: Valid state transition
- **WHEN** device in AUTHENTICATED state receives register action
- **THEN** system transitions device to REGISTERED state

#### Scenario: Invalid state transition
- **WHEN** device in PENDING state receives activate action
- **THEN** system rejects transition with 400 and error message

#### Scenario: State transition audit
- **WHEN** any state transition occurs
- **THEN** system logs transition with timestamp, from_state, to_state, trigger

### Requirement: Device Types
The system SHALL support multiple device types: PhysicalHost, Container, NetworkDevice, LoadBalancer, CloudInstance, IoT_Device.

#### Scenario: Register PhysicalHost
- **WHEN** user registers device with type PhysicalHost and SSH credentials
- **THEN** system creates device in PENDING state

#### Scenario: Register NetworkDevice
- **WHEN** user registers device with type NetworkDevice and SNMP configuration
- **THEN** system creates device in PENDING state

### Requirement: Device Hierarchy
The system SHALL support parent-child relationships between devices.

#### Scenario: Create device hierarchy
- **WHEN** user creates PhysicalHost, then adds Container as child
- **THEN** system establishes parent-child relationship

#### Scenario: Query children
- **WHEN** user requests GET /api/devices/:id/children
- **THEN** system returns all direct children of device

### Requirement: Device Groups
The system SHALL support flat, hierarchical, and dynamic device grouping.

#### Scenario: Flat group membership
- **WHEN** device is tagged with label env:prod
- **THEN** device belongs to flat group matching that tag

#### Scenario: Hierarchical group inheritance
- **WHEN** child group inherits from parent group
- **THEN** devices in child group inherit parent's labels

#### Scenario: Dynamic group membership
- **WHEN** device attribute changes to match dynamic group criteria
- **THEN** system automatically adds device to dynamic group

### Requirement: Configuration Templates
The system SHALL support Jinja2-style configuration templates with inheritance.

#### Scenario: Apply base template
- **WHEN** device requests configuration
- **THEN** system applies base template with device variables

#### Scenario: Template inheritance
- **WHEN** device type has type-specific template override
- **THEN** system merges override with base template (override wins)

### Requirement: Device Search
The system SHALL support searching devices by tags via GET /api/devices/search?tag=label=value.

#### Scenario: Search by tag
- **WHEN** user searches with tag=env:prod
- **THEN** system returns all devices with env:prod label

#### Scenario: Search with multiple tags
- **WHEN** user searches with tag=env:prod&tag=role:web
- **THEN** system returns devices matching ALL specified tags

### Requirement: Device Actions
The system SHALL support executing actions on devices via POST /api/devices/:id/actions.

#### Scenario: Execute action
- **WHEN** user sends action request with action type and parameters
- **THEN** system executes action on device and returns result
