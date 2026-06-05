# physical-host-project-linking

## Purpose

Define the many-to-many relationship between physical hosts and projects: a host can serve multiple projects (e.g. a database host serving order-backend and payment-gateway), and a project can span multiple hosts. This is the resource-allocation data model that the cost-allocation feature is built on. Hosts in maintenance are still shown linked to projects but flagged with maintenance status.

## Requirements

### Requirement: Physical Host Project Links
The system SHALL display and manage project associations for physical hosts.

#### Scenario: List projects linked to a physical host
- **WHEN** user opens physical host detail page and clicks "关联项目" tab
- **THEN** system displays all projects that this host is linked to
- **AND** each project shows: name, system, business line, link date

#### Scenario: Unlink host from project
- **WHEN** user clicks unlink button on a project association
- **THEN** system removes the resource link
- **AND** host no longer appears in project's resource list
- **AND** audit log entry is created

### Requirement: Project Physical Host Resources
The system SHALL display physical hosts in project resource lists.

#### Scenario: List physical hosts in project resources
- **WHEN** user opens project detail page and clicks "资源" tab
- **THEN** system displays all linked physical hosts
- **AND** each host shows: name, IP, state, type
- **AND** link date and weight are shown

#### Scenario: Link physical host to project from project page
- **WHEN** user clicks "添加物理主机" and selects a host
- **THEN** system creates resource link with weight=1.0
- **AND** host appears in project resource list

#### Scenario: Jump to host detail from project
- **WHEN** user clicks on a physical host in project resources
- **THEN** system navigates to physical host detail page
- **AND** host detail page shows this project in linked projects

### Requirement: Host Detail Shows Linked Projects
The system SHALL show which projects a physical host belongs to.

#### Scenario: Host detail linked projects tab
- **WHEN** user views a physical host with ID
- **THEN** "关联项目" tab shows all projects
- **AND** each project is clickable, navigating to project detail

#### Scenario: No linked projects
- **WHEN** user views a physical host with no project associations
- **THEN** "关联项目" tab shows empty state with message "该主机未关联任何项目"
