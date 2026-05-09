## ADDED Requirements

### Requirement: Resource Weight Allocation

The system SHALL support setting weight percentages for resources linked to multiple projects.

#### Scenario: Set weight when linking resource to project
- **WHEN** user links a resource (device/physical_host/pipeline) to a project
- **THEN** system accepts a weight value between 0.0 and 1.0
- **AND** default weight is 1.0 (100%) if not specified

#### Scenario: Edit weight of existing link
- **WHEN** user edits the weight of an existing project-resource link
- **THEN** system validates weight is between 0.0 and 1.0
- **AND** system saves the new weight
- **AND** audit log entry is created

#### Scenario: View total weight allocation for a resource
- **WHEN** user views a resource detail page (e.g., physical host)
- **THEN** system displays all linked projects with their weights
- **AND** system shows total weight sum (to detect over/under-allocation)

#### Scenario: Weight validation warning
- **WHEN** user sets weight and total exceeds 1.0 (100%)
- **THEN** system shows a warning message
- **AND** allows saving anyway (user may fix later)

#### Scenario: Weight validation for zero allocation
- **WHEN** user views a resource with total weight < 1.0
- **THEN** system shows remaining percentage as "unallocated"

### Requirement: Multi-Project Resource Display

The system SHALL display resources that are linked to multiple projects with their respective weights.

#### Scenario: List project resources with weights
- **WHEN** user opens project detail page and navigates to "Resources" tab
- **THEN** each resource shows its weight percentage
- **AND** resources with weight 0 are shown but dimmed

#### Scenario: Resource appears in multiple projects
- **WHEN** a resource is linked to multiple projects
- **AND** user views the resource from Project A
- **THEN** the resource shows Project A's weight
- **AND** viewing from Project B shows Project B's weight