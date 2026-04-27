# rbac-permissions

## ADDED Requirements

### Requirement: Role-Based Access Control
The system SHALL enforce RBAC with four roles: SuperAdmin, Operator, Developer, Auditor.

#### Scenario: SuperAdmin has all permissions
- **WHEN** SuperAdmin attempts any operation
- **THEN** system allows operation

#### Scenario: Operator has deploy and config permissions
- **WHEN** Operator attempts deploy or config-manage operation
- **THEN** system allows operation

#### Scenario: Operator restricted on production restart
- **WHEN** Operator attempts to restart production device
- **THEN** system denies with 403

#### Scenario: Developer has read and test-deploy permissions
- **WHEN** Developer attempts read operation
- **THEN** system allows operation

#### Scenario: Developer denied modify operations
- **WHEN** Developer attempts config modification
- **THEN** system denies with 403

#### Scenario: Auditor has read-only access
- **WHEN** Auditor attempts any write operation
- **THEN** system denies with 403

### Requirement: Permission Matrix
The system SHALL enforce the following permission matrix:

| Role | View Devices | Modify Config | Execute Commands | Remote Restart |
|------|-------------|--------------|------------------|----------------|
| Auditor | ✅ | ❌ | ❌ | ❌ |
| Developer | ✅ | ❌ | ❌ | ❌ |
| Operator | ✅ | ✅ | ✅ | ❌* |
| SuperAdmin | ✅ | ✅ | ✅ | ✅ |

*Operator can restart non-production devices only

### Requirement: Label-Based Access Control
The system SHALL enforce access control based on device labels.

#### Scenario: User accesses device with matching label
- **WHEN** user's group matches device label group
- **THEN** system allows access

#### Scenario: User accesses device without matching label
- **WHEN** user's group does not match device label group
- **THEN** system denies access with 403

### Requirement: Label Inheritance
The system SHALL support automatic label inheritance in hierarchical groups.

#### Scenario: Child group inherits parent labels
- **WHEN** device belongs to child group
- **THEN** device inherits all labels from parent group

### Requirement: Environment Restrictions
The system SHALL enforce environment-based restrictions.

#### Scenario: Operator restricted on prod devices
- **WHEN** Operator attempts production device modification
- **THEN** system denies with 403

### Requirement: Permission Middleware
The system SHALL use middleware to enforce RBAC on all protected endpoints.

#### Scenario: Request without token
- **WHEN** request to protected endpoint has no auth token
- **THEN** system returns 401

#### Scenario: Request with invalid token
- **WHEN** request to protected endpoint has invalid auth token
- **THEN** system returns 401

#### Scenario: Request with valid token but insufficient permissions
- **WHEN** request to protected endpoint has valid token but insufficient role
- **THEN** system returns 403
