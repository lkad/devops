# PRD v1.4 Engineering Supplement

**Supplements**: PRD.md v1.4
**Version**: 1.0
**Last Updated**: 2026-04-27

---

## 1. Data Models (Go Structs)

All models use `github.com/google/uuid` for UUIDs and `time.Time` for timestamps.

### 1.1 Device Management

```go
// internal/device/models.go

type DeviceType string

const (
    DeviceTypePhysicalHost  DeviceType = "physical_host"
    DeviceTypeContainer    DeviceType = "container"
    DeviceTypeNetworkDevice DeviceType = "network_device"
    DeviceTypeLoadBalancer  DeviceType = "load_balancer"
    DeviceTypeCloudInstance DeviceType = "cloud_instance"
    DeviceTypeIoTDevice    DeviceType = "iot_device"
)

type DeviceState string

const (
    DeviceStatePending       DeviceState = "pending"
    DeviceStateAuthenticated DeviceState = "authenticated"
    DeviceStateRegistered    DeviceState = "registered"
    DeviceStateActive        DeviceState = "active"
    DeviceStateMaintenance   DeviceState = "maintenance"
    DeviceStateSuspended     DeviceState = "suspended"
    DeviceStateRetire        DeviceState = "retire"
)

type Device struct {
    ID             uuid.UUID  `json:"id"`
    Name           string     `json:"name"`
    Type           DeviceType `json:"type"`
    State          DeviceState `json:"state"`
    ParentID       *uuid.UUID `json:"parentId,omitempty"`
    Metadata       JSONMap    `json:"metadata,omitempty"`
    Labels         JSONMap    `json:"labels,omitempty"`
    ConfigTemplate string     `json:"configTemplate,omitempty"`
    CreatedAt      time.Time  `json:"createdAt"`
    UpdatedAt      time.Time  `json:"updatedAt"`
}

type JSONMap map[string]interface{}

type DeviceStateTransition struct {
    ID         uuid.UUID    `json:"id"`
    DeviceID   uuid.UUID    `json:"deviceId"`
    FromState  DeviceState  `json:"fromState"`
    ToState    DeviceState  `json:"toState"`
    TriggeredBy string      `json:"triggeredBy"`
    Reason     string       `json:"reason,omitempty"`
    CreatedAt  time.Time    `json:"createdAt"`
}

type DeviceGroup struct {
    ID       uuid.UUID `json:"id"`
    Name     string    `json:"name"`
    ParentID *uuid.UUID `json:"parentId,omitempty"`
    Type     string    `json:"type"` // "flat", "hierarchical", "dynamic"
    Criteria JSONMap   `json:"criteria,omitempty"` // For dynamic groups
}

// DeviceState transitions validation
var ValidDeviceTransitions = map[DeviceState][]DeviceState{
    DeviceStatePending:       {DeviceStateAuthenticated},
    DeviceStateAuthenticated: {DeviceStateRegistered},
    DeviceStateRegistered:    {DeviceStateActive},
    DeviceStateActive:        {DeviceStateMaintenance, DeviceStateSuspended},
    DeviceStateMaintenance:   {DeviceStateActive},
    DeviceStateSuspended:     {DeviceStateActive, DeviceStateRetire},
}
```

### 1.2 CI/CD Pipeline

```go
// internal/cicd/models.go

type PipelineStatus string

const (
    PipelineStatusIdle     PipelineStatus = "idle"
    PipelineStatusRunning  PipelineStatus = "running"
    PipelineStatusSuccess PipelineStatus = "success"
    PipelineStatusFailed   PipelineStatus = "failed"
    PipelineStatusCancelled PipelineStatus = "cancelled"
)

type Pipeline struct {
    ID          uuid.UUID      `json:"id"`
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    YAMLConfig  string         `json:"yamlConfig"`
    Status      PipelineStatus `json:"status"`
    CreatedBy   string        `json:"createdBy"`
    CreatedAt   time.Time     `json:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"`
}

type PipelineRun struct {
    ID         uuid.UUID      `json:"id"`
    PipelineID  uuid.UUID      `json:"pipelineId"`
    Status     RunStatus      `json:"status"`
    StartedAt  *time.Time     `json:"startedAt,omitempty"`
    FinishedAt *time.Time     `json:"finishedAt,omitempty"`
    Trigger    string         `json:"trigger"` // "manual", "webhook", "scheduled"
    CreatedAt  time.Time      `json:"createdAt"`
}

type RunStatus string

const (
    RunStatusPending  RunStatus = "pending"
    RunStatusRunning  RunStatus = "running"
    RunStatusSuccess RunStatus = "success"
    RunStatusFailed  RunStatus = "failed"
    RunStatusCancelled RunStatus = "cancelled"
)

type PipelineRunStage struct {
    ID         uuid.UUID  `json:"id"`
    RunID      uuid.UUID  `json:"runId"`
    StageName  string    `json:"stageName"`
    Status     StageStatus `json:"status"`
    StartedAt  *time.Time `json:"startedAt,omitempty"`
    FinishedAt *time.Time `json:"finishedAt,omitempty"`
    Logs       string    `json:"logs,omitempty"`
    Error      string    `json:"error,omitempty"`
}

type StageStatus string

const (
    StageStatusPending StageStatus = "pending"
    StageStatusRunning StageStatus = "running"
    StageStatusSuccess StageStatus = "success"
    StageStatusFailed  StageStatus = "failed"
    StageStatusSkipped StageStatus = "skipped"
)

// Deployment strategies
type DeploymentStrategy string

const (
    StrategyBlueGreen DeploymentStrategy = "blue_green"
    StrategyCanary    DeploymentStrategy = "canary"
    StrategyRolling   DeploymentStrategy = "rolling"
)

// YAML Config Structure (parsed from YAML)
type PipelineConfig struct {
    Stages    []StageConfig `yaml:"stages"`
    Strategy  StrategyConfig `yaml:"strategy"`
}

type StageConfig struct {
    Name        string            `yaml:"name"`
    Commands    []string          `yaml:"commands"`
    Env        map[string]string `yaml:"env"`
    Timeout    string             `yaml:"timeout"` // e.g., "5m"
}

type StrategyConfig struct {
    Type      DeploymentStrategy `yaml:"type"`
    BlueGreen *BlueGreenConfig `yaml:"blue_green,omitempty"`
    Canary    *CanaryConfig    `yaml:"canary,omitempty"`
    Rolling   *RollingConfig   `yaml:"rolling,omitempty"`
}

type BlueGreenConfig struct {
    TargetGroup string `yaml:"targetGroup"`
}

type CanaryConfig struct {
    Stages []CanaryStage `yaml:"stages"` // e.g., [1%, 5%, 25%, 100%]
}

type CanaryStage struct {
    Percentage int `yaml:"percentage"`
    Duration   string `yaml:"duration"` // e.g., "10m"
}

type RollingConfig struct {
    MaxSurge       int `yaml:"max_surge"`       // Percentage
    MaxUnavailable int `yaml:"max_unavailable"` // Percentage
}
```

### 1.3 Logging System

```go
// internal/logs/models.go

type LogLevel string

const (
    LogLevelDebug LogLevel = "debug"
    LogLevelInfo  LogLevel = "info"
    LogLevelWarn  LogLevel = "warn"
    LogLevelError LogLevel = "error"
    LogLevelFatal LogLevel = "fatal"
)

type LogEntry struct {
    ID        uuid.UUID `json:"id"`
    Level     LogLevel  `json:"level"`
    Message   string    `json:"message"`
    Source    string    `json:"source,omitempty"` // "app", "device", "container"
    Metadata  JSONMap   `json:"metadata,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

type LogFilter struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    UserID    string    `json:"userId,omitempty"`
    Query     LogQuery  `json:"query"`
    CreatedAt time.Time `json:"createdAt"`
}

type LogQuery struct {
    Level    []LogLevel  `json:"level,omitempty"`
    Start    *time.Time `json:"start,omitempty"`
    End      *time.Time `json:"end,omitempty"`
    Search   string      `json:"search,omitempty"`
    Source   string      `json:"source,omitempty"`
}

type LogStats struct {
    Total     int            `json:"total"`
    ByLevel  map[string]int `json:"byLevel"`
    ByDay    map[string]int `json:"byDay"` // "2026-04-27": 150
}

type RetentionPolicy struct {
    Backend     string `json:"backend"` // "local", "elasticsearch", "loki"
    RetentionDays int  `json:"retentionDays"`
}

type StorageBackendType string

const (
    BackendLocal         StorageBackendType = "local"
    BackendElasticsearch StorageBackendType = "elasticsearch"
    BackendLoki         StorageBackendType = "loki"
)
```

### 1.4 Metrics Collection

```go
// internal/metrics/models.go

type MetricType string

const (
    MetricTypeCounter   MetricType = "counter"
    MetricTypeGauge     MetricType = "gauge"
    MetricTypeHistogram MetricType = "histogram"
)

type Metric struct {
    Name   string            `json:"name"`
    Type   MetricType        `json:"type"`
    Value float64           `json:"value"`
    Labels map[string]string `json:"labels,omitempty"`
}

type Counter struct {
    Name   string            `json:"name"`
    Value  float64           `json:"value"`
    Labels map[string]string `json:"labels,omitempty"`
}

type Gauge struct {
    Name   string            `json:"name"`
    Value  float64           `json:"value"`
    Labels map[string]string `json:"labels,omitempty"`
}

type Histogram struct {
    Name   string            `json:"name"`
    Value  float64           `json:"value"`
    Labels map[string]string `json:"labels,omitempty"`
}

// Available metrics (as defined in PRD)
var DefaultMetrics = []struct {
    Name  string
    Type  MetricType
    Help  string
}{
    {"devops_toolkit_info", MetricTypeGauge, "System info"},
    {"http_requests_total", MetricTypeCounter, "HTTP request count"},
    {"http_request_duration_ms", MetricTypeHistogram, "HTTP request latency"},
    {"logs_total", MetricTypeCounter, "Log count by level"},
    {"device_events_total", MetricTypeCounter, "Device events"},
    {"pipeline_events_total", MetricTypeCounter, "Pipeline events"},
    {"alerts_total", MetricTypeCounter, "Alert count"},
}
```

### 1.5 Alert Notification

```go
// internal/alerts/models.go

type ChannelType string

const (
    ChannelTypeSlack   ChannelType = "slack"
    ChannelTypeWebhook ChannelType = "webhook"
    ChannelTypeEmail   ChannelType = "email"
    ChannelTypeLog     ChannelType = "log"
)

type AlertSeverity string

const (
    AlertSeverityCritical AlertSeverity = "critical"
    AlertSeverityWarning  AlertSeverity = "warning"
    AlertSeverityInfo    AlertSeverity = "info"
)

type AlertChannel struct {
    ID        uuid.UUID      `json:"id"`
    Name      string         `json:"name"`
    Type      ChannelType    `json:"type"`
    Config    ChannelConfig  `json:"config"`
    CreatedAt time.Time     `json:"createdAt"`
}

type ChannelConfig map[string]interface{}

// Slack config: { "webhookUrl": "https://...", "channel": "#alerts" }
// Webhook config: { "url": "https://...", "headers": {...} }
// Email config: { "recipients": ["a@b.com", "c@d.com"] }

type AlertHistory struct {
    ID          uuid.UUID      `json:"id"`
    Name        string         `json:"name"`
    Severity    AlertSeverity  `json:"severity"`
    Message     string         `json:"message"`
    Channel     string         `json:"channel"`
    Status      AlertStatus    `json:"status"`
    TriggeredBy string         `json:"triggeredBy,omitempty"`
    CreatedAt   time.Time     `json:"createdAt"`
}

type AlertStatus string

const (
    AlertStatusSent       AlertStatus = "sent"
    AlertStatusRateLimited AlertStatus = "rate_limited"
    AlertStatusFailed     AlertStatus = "failed"
)

type AlertStats struct {
    Total      int            `json:"total"`
    BySeverity map[string]int `json:"bySeverity"`
    ByName     map[string]int `json:"byName"`
    Last24h    int            `json:"last24h"`
}

// Rate limiting config
type RateLimitConfig struct {
    WindowSeconds int `json:"windowSeconds"` // 60
    MaxAlerts     int `json:"maxAlerts"`     // 10
}
```

### 1.6 LDAP Authentication

```go
// internal/auth/models.go

type UserRole string

const (
    RoleSuperAdmin UserRole = "super_admin"
    RoleOperator   UserRole = "operator"
    RoleDeveloper UserRole = "developer"
    RoleAuditor   UserRole = "auditor"
)

type User struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    Email     string    `json:"email"`
    Role      UserRole  `json:"role"`
    LDAPDN    string    `json:"ldapDn,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}

type JWTClaims struct {
    UserID   uuid.UUID `json:"userId"`
    Username string    `json:"username"`
    Role     UserRole  `json:"role"`
    ExpiresAt time.Time `json:"expiresAt"`
}

type LDAPConfig struct {
    Host         string            `json:"host"`
    Port         int               `json:"port"` // 389
    BaseDN       string            `json:"baseDn"`
    BindDN       string            `json:"bindDn"`
    BindPassword string            `json:"bindPassword,omitempty"`
    UserFilter   string            `json:"userFilter"` // "(uid=%s)"
    GroupMapping map[string]UserRole `json:"groupMapping"` // DN -> Role
}

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expiresAt"`
    User      User      `json:"user"`
}
```

### 1.7 RBAC Permissions

```go
// internal/rbac/models.go

type Permission string

const (
    // Device permissions
    PermViewDevices   Permission = "devices:view"
    PermModifyConfig  Permission = "devices:config"
    PermExecuteCmd    Permission = "devices:execute"
    PermRestartDevice Permission = "devices:restart"

    // Pipeline permissions
    PermViewPipelines   Permission = "pipelines:view"
    PermCreatePipeline  Permission = "pipelines:create"
    PermExecutePipeline Permission = "pipelines:execute"

    // Project permissions
    PermViewProjects   Permission = "projects:view"
    PermEditProjects  Permission = "projects:edit"
    PermAdminProjects Permission = "projects:admin"
)

type RolePermissions struct {
    Role       UserRole       `json:"role"`
    Permissions []Permission  `json:"permissions"`
}

// Permission matrix (from PRD Section 10.2)
var RolePermissionMatrix = map[UserRole][]Permission{
    RoleAuditor: {
        PermViewDevices,
        PermViewPipelines,
        PermViewProjects,
    },
    RoleDeveloper: {
        PermViewDevices,
        PermViewPipelines,
        PermExecutePipeline,
        PermViewProjects,
    },
    RoleOperator: {
        PermViewDevices,
        PermModifyConfig,
        PermExecuteCmd,
        // PermRestartDevice - only non-production
        PermViewPipelines,
        PermCreatePipeline,
        PermExecutePipeline,
        PermViewProjects,
        PermEditProjects,
    },
    RoleSuperAdmin: {
        PermViewDevices,
        PermModifyConfig,
        PermExecuteCmd,
        PermRestartDevice,
        PermViewPipelines,
        PermCreatePipeline,
        PermExecutePipeline,
        PermViewProjects,
        PermEditProjects,
        PermAdminProjects,
    },
}

type AccessDecision struct {
    Allowed   bool     `json:"allowed"`
    Reason    string   `json:"reason,omitempty"`
    Failures []string `json:"failures,omitempty"` // ["production_restricted"]
}

type LabelAccess struct {
    UserGroups []string `json:"userGroups"`
    DeviceLabels JSONMap `json:"deviceLabels"`
    Environment string    `json:"environment"` // "prod", "dev", "test"
}
```

### 1.8 Project Hierarchy

```go
// internal/project/models.go

type ProjectType string

const (
    ProjectTypeFrontend ProjectType = "frontend"
    ProjectTypeBackend  ProjectType = "backend"
)

type ProjectRole string

const (
    ProjectRoleViewer  ProjectRole = "viewer"
    ProjectRoleEditor  ProjectRole = "editor"
    ProjectRoleAdmin   ProjectRole = "admin"
)

type BusinessLine struct {
    ID          uuid.UUID `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

type System struct {
    ID            uuid.UUID `json:"id"`
    BusinessLineID uuid.UUID `json:"businessLineId"`
    Name          string    `json:"name"`
    Description   string    `json:"description,omitempty"`
    CreatedAt     time.Time `json:"createdAt"`
    UpdatedAt     time.Time `json:"updatedAt"`
}

type Project struct {
    ID        uuid.UUID   `json:"id"`
    SystemID  uuid.UUID   `json:"systemId"`
    Name      string      `json:"name"`
    Type      ProjectType `json:"type"`
    CreatedAt time.Time   `json:"createdAt"`
    UpdatedAt time.Time   `json:"updatedAt"`
}

type ResourceType string

const (
    ResourceTypeDevice       ResourceType = "device"
    ResourceTypePipeline     ResourceType = "pipeline"
    ResourceTypeLogSource   ResourceType = "log_source"
    ResourceTypeAlertChannel ResourceType = "alert_channel"
    ResourceTypePhysicalHost ResourceType = "physical_host"
)

type ProjectResource struct {
    ID           uuid.UUID     `json:"id"`
    ProjectID    uuid.UUID    `json:"projectId"`
    ResourceType ResourceType `json:"resourceType"`
    ResourceID   uuid.UUID    `json:"resourceId"`
    CreatedAt    time.Time     `json:"createdAt"`
}

type ProjectPermission struct {
    ID        uuid.UUID   `json:"id"`
    ProjectID uuid.UUID   `json:"projectId"`
    Username  string     `json:"username"`
    Role      ProjectRole `json:"role"`
    GrantedBy string     `json:"grantedBy,omitempty"`
    CreatedAt time.Time   `json:"createdAt"`
}

// FinOps report row
type FinOpsRow struct {
    BusinessLine  string `json:"businessLine"`
    System        string `json:"system"`
    ProjectType   string `json:"projectType"`
    Project       string `json:"project"`
    ResourceType  string `json:"resourceType"`
    Count         int    `json:"count"`
    Unit          string `json:"unit"`
}
```

### 1.9 Physical Host Monitoring

```go
// internal/physicalhost/models.go

type HostState string

const (
    HostStateOnline  HostState = "online"
    HostStateOffline HostState = "offline"
)

type DataStatus string

const (
    DataStatusFresh       DataStatus = "fresh"
    DataStatusStale       DataStatus = "stale"
    DataStatusUnavailable DataStatus = "unavailable"
)

type PhysicalHost struct {
    ID               uuid.UUID  `json:"id"`
    Hostname         string     `json:"hostname"`
    IP               string     `json:"ip"`
    Port             int        `json:"port"` // default 22
    State            HostState  `json:"state"`
    LastHeartbeat    *time.Time `json:"lastHeartbeat,omitempty"`
    LastAgentUpdate  *time.Time `json:"lastAgentUpdate,omitempty"`
    DataStatus       DataStatus `json:"dataStatus"`
    SSHUser          string     `json:"sshUser,omitempty"`
    RegisteredAt     time.Time  `json:"registeredAt"`
}

type HostMetrics struct {
    CPU    CPUMetrics   `json:"cpu"`
    Memory MemoryMetrics `json:"memory"`
    Disk   []DiskMetrics `json:"disk"`
    Uptime UptimeMetrics `json:"uptime"`
}

type CPUMetrics struct {
    Usage    float64 `json:"usage"`    // percentage
    Cores    int     `json:"cores"`
    User     float64 `json:"user"`     // percentage
    System   float64 `json:"system"`   // percentage
    Idle     float64 `json:"idle"`     // percentage
}

type MemoryMetrics struct {
    Total       uint64  `json:"total"`        // MB
    Used        uint64  `json:"used"`        // MB
    Free        uint64  `json:"free"`        // MB
    UsagePercent float64 `json:"usagePercent"`
}

type DiskMetrics struct {
    Device       string  `json:"device"`
    Total        uint64  `json:"total"`     // GB
    Used         uint64  `json:"used"`      // GB
    Available    uint64  `json:"available"` // GB
    UsagePercent float64 `json:"usagePercent"`
}

type UptimeMetrics struct {
    Value     uint64 `json:"value"`      // seconds
    Formatted string `json:"formatted"`  // "10d 10h 0m"
}

type ServiceStatus struct {
    Name   string `json:"name"`
    Status string `json:"status"` // "running", "stopped", "unknown"
}

type HostSummary struct {
    Total   int `json:"total"`
    Online  int `json:"online"`
    Offline int `json:"offline"`
}

type CacheStatus struct {
    Size     int     `json:"size"`
    HitRate  float64 `json:"hitRate"`
}
```

### 1.10 K8s Cluster Management

```go
// internal/k8s/models.go

type ClusterStatus string

const (
    ClusterStatusActive   ClusterStatus = "active"
    ClusterStatusInactive ClusterStatus = "inactive"
    ClusterStatusError   ClusterStatus = "error"
)

type K8sCluster struct {
    ID        uuid.UUID     `json:"id"`
    Name      string        `json:"name"`
    Kubeconfig string      `json:"kubeconfig,omitempty"` // stored securely
    Status    ClusterStatus `json:"status"`
    CreatedAt time.Time     `json:"createdAt"`
    UpdatedAt time.Time     `json:"updatedAt"`
}

type NodeHealth struct {
    Name     string `json:"name"`
    Status   string `json:"status"` // "Ready", "NotReady"
    CPU      string `json:"cpu"`    // "15%" etc
    Memory   string `json:"memory"` // "32%" etc
    Age      string `json:"age"`
}

type Workload struct {
    Name         string `json:"name"`
    Namespace    string `json:"namespace"`
    Type         string `json:"type"` // "Deployment", "StatefulSet", etc
    Ready        string `json:"ready"` // "3/3"
    Age          string `json:"age"`
}

type PodLogs struct {
    PodName  string `json:"podName"`
    Logs     string `json:"logs"`
    Previous bool   `json:"previous"` // previous container logs (for restart)
}

type ExecResult struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}
```

### 1.11 Network Discovery

```go
// internal/discovery/models.go

type DiscoveredDeviceType string

const (
    DiscoveredTypePhysicalHost DiscoveredDeviceType = "physical_host"
    DiscoveredTypeNetworkDevice DiscoveredDeviceType = "network_device"
)

type Protocol string

const (
    ProtocolSSH Protocol = "ssh"
    ProtocolSNMP Protocol = "snmp"
)

type DiscoveredDevice struct {
    ID        uuid.UUID            `json:"id"`
    IP        string               `json:"ip"`
    Port      int                  `json:"port"`
    DeviceType DiscoveredDeviceType `json:"deviceType"`
    Protocol   Protocol             `json:"protocol"`
    Metadata  JSONMap             `json:"metadata,omitempty"`
    ScanID    *uuid.UUID           `json:"scanId,omitempty"`
    CreatedAt time.Time            `json:"createdAt"`
}

type ScanResult struct {
    ID           uuid.UUID         `json:"id"`
    Network      string            `json:"network"`
    Timeout      int               `json:"timeout"` // seconds
    DevicesFound int               `json:"devicesFound"`
    StartedAt    time.Time         `json:"startedAt"`
    FinishedAt   *time.Time        `json:"finishedAt,omitempty"`
}

type DiscoveryStatus struct {
    LastScan      *time.Time `json:"lastScan,omitempty"`
    DevicesFound  int        `json:"devicesFound"`
    Pending       int        `json:"pending"`
}

// Device ID naming convention (from PRD)
func GenerateDeviceID(nodeType, datacenter, name string, num int) string {
    // e.g., "clab-dc1-web-21"
    return fmt.Sprintf("clab-%s-%s-%s-%d", datacenter, strings.ToLower(nodeType), name, num)
}
```

### 1.12 Audit Logging

```go
// internal/audit/models.go

type AuditAction string

const (
    AuditActionCreate AuditAction = "create"
    AuditActionUpdate AuditAction = "update"
    AuditActionDelete AuditAction = "delete"
)

type AuditEntityType string

const (
    AuditEntityBusinessLine AuditEntityType = "business_line"
    AuditEntitySystem       AuditEntityType = "system"
    AuditEntityProject       AuditEntityType = "project"
    AuditEntityResourceLink AuditEntityType = "resource_link"
    AuditEntityPermission    AuditEntityType = "permission"
)

type AuditLog struct {
    ID         uuid.UUID        `json:"id"`
    Username   string           `json:"username"`
    Action     AuditAction      `json:"action"`
    EntityType AuditEntityType  `json:"entityType"`
    EntityID   uuid.UUID        `json:"entityId"`
    EntityName string           `json:"entityName,omitempty"`
    Changes    string           `json:"changes,omitempty"`
    OldValue   string           `json:"oldValue,omitempty"`
    NewValue   string           `json:"newValue,omitempty"`
    IPAddress  string           `json:"ipAddress,omitempty"`
    CreatedAt  time.Time        `json:"createdAt"`
}

type AuditLogQuery struct {
    EntityType *AuditEntityType `json:"entityType,omitempty"`
    EntityID   *uuid.UUID       `json:"entityId,omitempty"`
    Username   *string          `json:"username,omitempty"`
    Action     *AuditAction     `json:"action,omitempty"`
    Start      *time.Time       `json:"start,omitempty"`
    End        *time.Time       `json:"end,omitempty"`
    Limit      int              `json:"limit"`  // default 20
    Offset     int              `json:"offset"` // default 0
}
```

### 1.13 WebSocket

```go
// internal/websocket/models.go

type WSChannel string

const (
    WSChannelLog          WSChannel = "log"
    WSChannelMetric       WSChannel = "metric"
    WSChannelDeviceEvent  WSChannel = "device_event"
    WSChannelPipelineUpdate WSChannel = "pipeline_update"
    WSChannelAlert        WSChannel = "alert"
)

type WSMessage struct {
    Channel   WSChannel   `json:"channel"`
    Type      string      `json:"type"`
    Data      interface{} `json:"data"`
    Timestamp time.Time   `json:"timestamp"`
}

type WSSubscription struct {
    Action  string    `json:"action"` // "subscribe", "unsubscribe"
    Channel WSChannel `json:"channel"`
}

type WSClientMessage struct {
    Action  string    `json:"action"`
    Channel WSChannel `json:"channel,omitempty"`
}
```

---

## 2. Database Schema (PostgreSQL)

### 2.1 Migration Files

#### 000001_initial_schema.up.sql

```sql
-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- USERS AND AUTHENTICATION
-- ============================================

CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(255) UNIQUE NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    role        VARCHAR(50) NOT NULL CHECK (role IN ('super_admin', 'operator', 'developer', 'auditor')),
    ldap_dn     VARCHAR(500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_role ON users(role);

-- ============================================
-- DEVICES
-- ============================================

CREATE TABLE devices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50) NOT NULL CHECK (type IN ('physical_host', 'container', 'network_device', 'load_balancer', 'cloud_instance', 'iot_device')),
    state           VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'authenticated', 'registered', 'active', 'maintenance', 'suspended', 'retire')),
    parent_id       UUID REFERENCES devices(id) ON DELETE SET NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    labels          JSONB NOT NULL DEFAULT '{}',
    config_template TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_state ON devices(state);
CREATE INDEX idx_devices_type ON devices(type);
CREATE INDEX idx_devices_labels ON devices USING GIN(labels);
CREATE INDEX idx_devices_parent ON devices(parent_id);

-- Device state transition log
CREATE TABLE device_state_transitions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id     UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    from_state    VARCHAR(50) NOT NULL,
    to_state      VARCHAR(50) NOT NULL,
    triggered_by  VARCHAR(255),
    reason        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dst_device ON device_state_transitions(device_id);
CREATE INDEX idx_dst_created ON device_state_transitions(created_at);

-- Device groups
CREATE TABLE device_groups (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name      VARCHAR(255) NOT NULL,
    parent_id  UUID REFERENCES device_groups(id) ON DELETE CASCADE,
    type      VARCHAR(50) NOT NULL CHECK (type IN ('flat', 'hierarchical', 'dynamic')),
    criteria  JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dg_parent ON device_groups(parent_id);

-- Device group membership
CREATE TABLE device_group_members (
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    group_id  UUID NOT NULL REFERENCES device_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (device_id, group_id)
);

-- ============================================
-- CI/CD PIPELINES
-- ============================================

CREATE TABLE pipelines (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    yaml_config TEXT NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'success', 'failed', 'cancelled')),
    created_by  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pipelines_status ON pipelines(status);
CREATE INDEX idx_pipelines_created_by ON pipelines(created_by);

CREATE TABLE pipeline_runs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_id UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    status      VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled')),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    trigger     VARCHAR(50) NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pr_pipeline ON pipeline_runs(pipeline_id);
CREATE INDEX idx_pr_status ON pipeline_runs(status);
CREATE INDEX idx_pr_created ON pipeline_runs(created_at);

CREATE TABLE pipeline_run_stages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id      UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    stage_name  VARCHAR(100) NOT NULL,
    status      VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'success', 'failed', 'skipped')),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    logs        TEXT,
    error       TEXT
);

CREATE INDEX idx_prs_run ON pipeline_run_stages(run_id);

-- ============================================
-- LOGGING
-- ============================================

CREATE TABLE log_entries (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    level       VARCHAR(20) NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error', 'fatal')),
    message     TEXT NOT NULL,
    source      VARCHAR(100),
    metadata    JSONB DEFAULT '{}',
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_le_level ON log_entries(level);
CREATE INDEX idx_le_source ON log_entries(source);
CREATE INDEX idx_le_timestamp ON log_entries(timestamp DESC);
-- For efficient time-range queries
CREATE INDEX idx_le_timestamp_range ON log_entries(timestamp DESC, id);

CREATE TABLE log_filters (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name      VARCHAR(255) NOT NULL,
    user_id   VARCHAR(255),
    query     JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- ALERTS
-- ============================================

CREATE TABLE alert_channels (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(255) UNIQUE NOT NULL,
    type       VARCHAR(50) NOT NULL CHECK (type IN ('slack', 'webhook', 'email', 'log')),
    config     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ac_type ON alert_channels(type);

CREATE TABLE alert_history (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(255) NOT NULL,
    severity     VARCHAR(20) NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    message      TEXT NOT NULL,
    channel      VARCHAR(255) NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'sent' CHECK (status IN ('sent', 'rate_limited', 'failed')),
    triggered_by VARCHAR(255),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ah_name ON alert_history(name);
CREATE INDEX idx_ah_severity ON alert_history(severity);
CREATE INDEX idx_ah_created ON alert_history(created_at DESC);

-- ============================================
-- PHYSICAL HOSTS
-- ============================================

CREATE TABLE physical_hosts (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hostname         VARCHAR(255) NOT NULL,
    ip               INET NOT NULL,
    port             INTEGER NOT NULL DEFAULT 22,
    state            VARCHAR(50) NOT NULL DEFAULT 'online' CHECK (state IN ('online', 'offline')),
    last_heartbeat   TIMESTAMPTZ,
    last_agent_update TIMESTAMPTZ,
    data_status      VARCHAR(50) NOT NULL DEFAULT 'fresh' CHECK (data_status IN ('fresh', 'stale', 'unavailable')),
    ssh_user         VARCHAR(255),
    ssh_password_enc BYTEA, -- encrypted
    registered_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ph_state ON physical_hosts(state);
CREATE INDEX idx_ph_ip ON physical_hosts(ip);

-- ============================================
-- K8S CLUSTERS
-- ============================================

CREATE TABLE k8s_clusters (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) UNIQUE NOT NULL,
    kubeconfig  TEXT NOT NULL,
    status      VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'error')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kc_status ON k8s_clusters(status);

-- ============================================
-- PROJECT HIERARCHY
-- ============================================

CREATE TABLE business_lines (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE systems (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_line_id UUID NOT NULL REFERENCES business_lines(id) ON DELETE CASCADE,
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sys_bl ON systems(business_line_id);

CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    system_id   UUID NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('frontend', 'backend')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_proj_system ON projects(system_id);

CREATE TABLE project_resources (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    resource_type VARCHAR(50) NOT NULL,
    resource_id   UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, resource_type, resource_id)
);

CREATE INDEX idx_pr_project ON project_resources(project_id);
CREATE INDEX idx_pr_resource ON project_resources(resource_type, resource_id);

CREATE TABLE project_permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    username    VARCHAR(255) NOT NULL,
    role        VARCHAR(50) NOT NULL CHECK (role IN ('viewer', 'editor', 'admin')),
    granted_by  VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, username)
);

CREATE INDEX idx_pp_project ON project_permissions(project_id);
CREATE INDEX idx_pp_username ON project_permissions(username);

-- ============================================
-- NETWORK DISCOVERY
-- ============================================

CREATE TABLE discovered_devices (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ip          INET NOT NULL,
    port        INTEGER,
    device_type VARCHAR(50) NOT NULL,
    protocol    VARCHAR(50) NOT NULL,
    metadata    JSONB DEFAULT '{}',
    scan_id     UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dd_scan ON discovered_devices(scan_id);
CREATE INDEX idx_dd_ip ON discovered_devices(ip);

CREATE TABLE discovery_scans (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    network       VARCHAR(100) NOT NULL,
    timeout       INTEGER DEFAULT 2000,
    devices_found INTEGER DEFAULT 0,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ
);

CREATE INDEX idx_ds_started ON discovery_scans(started_at DESC);

-- ============================================
-- AUDIT LOGGING
-- ============================================

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username    VARCHAR(255) NOT NULL,
    action      VARCHAR(50) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID NOT NULL,
    entity_name VARCHAR(255),
    changes     TEXT,
    old_value   TEXT,
    new_value   TEXT,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_al_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_al_username ON audit_logs(username);
CREATE INDEX idx_al_created ON audit_logs(created_at DESC);

-- ============================================
-- UPDATED_AT TRIGGER
-- ============================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply to all tables with updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_devices_updated_at BEFORE UPDATE ON devices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_pipelines_updated_at BEFORE UPDATE ON pipelines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_business_lines_updated_at BEFORE UPDATE ON business_lines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_systems_updated_at BEFORE UPDATE ON systems
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_k8s_clusters_updated_at BEFORE UPDATE ON k8s_clusters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

#### 000001_initial_schema.down.sql

```sql
-- Drop triggers first
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_devices_updated_at ON devices;
DROP TRIGGER IF EXISTS update_pipelines_updated_at ON pipelines;
DROP TRIGGER IF EXISTS update_business_lines_updated_at ON business_lines;
DROP TRIGGER IF EXISTS update_systems_updated_at ON systems;
DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS update_k8s_clusters_updated_at ON k8s_clusters;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS discovered_devices;
DROP TABLE IF EXISTS discovery_scans;
DROP TABLE IF EXISTS project_permissions;
DROP TABLE IF EXISTS project_resources;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS systems;
DROP TABLE IF EXISTS business_lines;
DROP TABLE IF EXISTS k8s_clusters;
DROP TABLE IF EXISTS physical_hosts;
DROP TABLE IF EXISTS alert_history;
DROP TABLE IF EXISTS alert_channels;
DROP TABLE IF EXISTS log_filters;
DROP TABLE IF EXISTS log_entries;
DROP TABLE IF EXISTS pipeline_run_stages;
DROP TABLE IF EXISTS pipeline_runs;
DROP TABLE IF EXISTS pipelines;
DROP TABLE IF EXISTS device_group_members;
DROP TABLE IF EXISTS device_groups;
DROP TABLE IF EXISTS device_state_transitions;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS users;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";
```

---

## 3. API Contract

### 3.1 Standard Response Format

```go
// All responses follow this structure
type Response struct {
    Data interface{} `json:"data"`
    Meta ResponseMeta `json:"meta"`
}

type ResponseMeta struct {
    RequestID  string      `json:"requestId"`
    Timestamp  string      `json:"timestamp"` // ISO8601
    Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
    Total  int  `json:"total"`
    Limit  int  `json:"limit"`
    Offset int  `json:"offset"`
    HasMore bool `json:"hasMore"`
}

type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
    Meta  ResponseMeta `json:"meta"`
}

type ErrorDetail struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}
```

### 3.2 Standard Error Codes

| Code | HTTP Status | Description | Example |
|------|-------------|-------------|---------|
| `VALIDATION_ERROR` | 400 | Invalid input parameters | `{"field": "email", "reason": "invalid format"}` |
| `UNAUTHORIZED` | 401 | Authentication required | `{"reason": "token expired"}` |
| `FORBIDDEN` | 403 | Insufficient permissions | `{"required": "admin", "actual": "viewer"}` |
| `NOT_FOUND` | 404 | Resource not found | `{"resource": "device", "id": "uuid"}` |
| `CONFLICT` | 409 | Resource conflict | `{"reason": "duplicate name"}` |
| `INVALID_STATE` | 422 | Invalid state transition | `{"current": "pending", "requested": "active"}` |
| `RATE_LIMITED` | 429 | Rate limit exceeded | `{"retryAfter": 60}` |
| `INTERNAL_ERROR` | 500 | Unexpected server error | `{"requestId": "uuid"}` |

### 3.3 Pagination Conventions

All list endpoints support pagination:

```go
// Request
GET /api/devices?limit=20&offset=0&sort=created_at&order=desc

// Response headers
X-Total-Count: 150
X-Request-Id: uuid
```

### 3.4 Filtering Conventions

```go
// Single filter
GET /api/devices?state=active

// Multiple filters (AND)
GET /api/devices?state=active&type=physical_host

// JSON filter for complex queries
GET /api/logs?filter={"level":["error","warn"]}

// Search
GET /api/devices?search=web-server
```

---

## 4. Complete API Request/Response Examples

### 4.1 Authentication

**POST /api/auth/login**

Request:
```json
{
    "username": "john.doe",
    "password": "secret123"
}
```

Response (200):
```json
{
    "data": {
        "token": "eyJhbGciOiJIUzI1NiIs...",
        "expiresAt": "2026-04-28T10:00:00Z",
        "user": {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "username": "john.doe",
            "email": "john.doe@example.com",
            "role": "operator"
        }
    },
    "meta": {
        "requestId": "req-123",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

Response (401):
```json
{
    "error": {
        "code": "UNAUTHORIZED",
        "message": "Invalid credentials"
    },
    "meta": {
        "requestId": "req-124",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

### 4.2 Device Management

**POST /api/devices**

Request:
```json
{
    "name": "web-server-01",
    "type": "physical_host",
    "labels": {
        "env": "production",
        "role": "web",
        "datacenter": "dc1"
    },
    "metadata": {
        "os": "ubuntu-22.04",
        "cpu": "8 cores"
    }
}
```

Response (201):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440001",
        "name": "web-server-01",
        "type": "physical_host",
        "state": "pending",
        "labels": {
            "env": "production",
            "role": "web",
            "datacenter": "dc1"
        },
        "metadata": {
            "os": "ubuntu-22.04",
            "cpu": "8 cores"
        },
        "createdAt": "2026-04-27T10:00:00Z",
        "updatedAt": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-125",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

**GET /api/devices**

Request:
```
GET /api/devices?limit=10&offset=0&state=active&type=physical_host
```

Response (200):
```json
{
    "data": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "name": "web-server-01",
            "type": "physical_host",
            "state": "active",
            "labels": {"env": "production", "role": "web"},
            "createdAt": "2026-04-27T10:00:00Z"
        }
    ],
    "meta": {
        "requestId": "req-126",
        "timestamp": "2026-04-27T10:00:00Z",
        "pagination": {
            "total": 1,
            "limit": 10,
            "offset": 0,
            "hasMore": false
        }
    }
}
```

**POST /api/devices/:id/actions**

Request:
```json
{
    "action": "restart",
    "params": {
        "force": false
    }
}
```

Response (200):
```json
{
    "data": {
        "action": "restart",
        "status": "completed",
        "output": "Service restarted successfully"
    },
    "meta": {
        "requestId": "req-127",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

### 4.3 Pipeline Management

**POST /api/pipelines**

Request:
```json
{
    "name": "my-app-deploy",
    "description": "Production deployment pipeline",
    "yamlConfig": "stages:\n  - name: build\n    commands:\n      - go build ./...\n  - name: test\n    commands:\n      - go test ./...\nstrategy:\n  type: rolling\n  max_surge: 20"
}
```

Response (201):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440002",
        "name": "my-app-deploy",
        "description": "Production deployment pipeline",
        "status": "idle",
        "createdBy": "john.doe",
        "createdAt": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-128",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

**POST /api/pipelines/:id/execute**

Response (202):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440003",
        "pipelineId": "550e8400-e29b-41d4-a716-446655440002",
        "status": "running",
        "startedAt": "2026-04-27T10:00:00Z",
        "trigger": "manual"
    },
    "meta": {
        "requestId": "req-129",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

### 4.4 Log Management

**POST /api/logs**

Request:
```json
{
    "level": "info",
    "message": "User logged in",
    "metadata": {
        "userId": "john.doe",
        "ip": "192.168.1.1"
    }
}
```

Response (201):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440004",
        "level": "info",
        "message": "User logged in",
        "metadata": {
            "userId": "john.doe",
            "ip": "192.168.1.1"
        },
        "timestamp": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-130",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

**GET /api/logs**

Request:
```
GET /api/logs?level=error&start=2026-04-01T00:00:00Z&end=2026-04-30T23:59:59Z&limit=50&offset=0
```

Response (200):
```json
{
    "data": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440005",
            "level": "error",
            "message": "Connection refused",
            "timestamp": "2026-04-27T09:30:00Z"
        }
    ],
    "meta": {
        "requestId": "req-131",
        "timestamp": "2026-04-27T10:00:00Z",
        "pagination": {
            "total": 25,
            "limit": 50,
            "offset": 0,
            "hasMore": false
        }
    }
}
```

### 4.5 Alert Management

**POST /api/alerts/channels**

Request:
```json
{
    "name": "slack-ops",
    "type": "slack",
    "config": {
        "webhookUrl": "https://hooks.slack.com/services/xxx",
        "channel": "#ops-alerts"
    }
}
```

Response (201):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440006",
        "name": "slack-ops",
        "type": "slack",
        "config": {
            "webhookUrl": "***",
            "channel": "#ops-alerts"
        },
        "createdAt": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-132",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

**POST /api/alerts/trigger**

Request:
```json
{
    "name": "high_cpu",
    "severity": "warning",
    "message": "CPU usage exceeded 80% on web-server-01",
    "channel": "slack-ops"
}
```

Response (200):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440007",
        "name": "high_cpu",
        "severity": "warning",
        "message": "CPU usage exceeded 80% on web-server-01",
        "channel": "slack-ops",
        "status": "sent",
        "createdAt": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-133",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

### 4.6 Project Hierarchy

**POST /api/org/business-lines**

Request:
```json
{
    "name": "E-Commerce Division",
    "description": "All e-commerce related systems"
}
```

Response (201):
```json
{
    "data": {
        "id": "550e8400-e29b-41d4-a716-446655440008",
        "name": "E-Commerce Division",
        "description": "All e-commerce related systems",
        "createdAt": "2026-04-27T10:00:00Z",
        "updatedAt": "2026-04-27T10:00:00Z"
    },
    "meta": {
        "requestId": "req-134",
        "timestamp": "2026-04-27T10:00:00Z"
    }
}
```

**GET /api/org/reports/finops?period=2026-04**

Response (200):
```csv
Business Line,System,Project Type,Project,Resource Type,Count,Unit
E-Commerce Division,Order System,Backend,order-backend,VM,3,nodes
E-Commerce Division,Order System,Backend,order-backend,Storage,500,GB
E-Commerce Division,Order System,Frontend,order-frontend,VM,2,nodes
```

---

## 5. Configuration Schema

```yaml
# config.yaml

app:
  name: "devops-toolkit"
  version: "1.0.0"
  host: "0.0.0.0"
  port: 8080
  env: "development"  # development, production

server:
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s

database:
  host: "localhost"
  port: 5432
  username: "devops"
  password: "${DB_PASSWORD}"  # from environment
  name: "devops"
  max_connections: 25
  ssl_mode: "disable"
  conn_max_lifetime: 1h

redis:
  host: "localhost"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0
  pool_size: 10

auth:
  jwt_secret: "${JWT_SECRET}"
  token_expiry: 24h
  ldap:
    enabled: true
    host: "ldap.example.com"
    port: 389
    base_dn: "dc=example,dc=com"
    bind_dn: "cn=admin,dc=example,dc=com"
    bind_password: "${LDAP_BIND_PASSWORD}"
    user_filter: "(uid=%s)"
    group_mapping:
      "cn=IT_Ops,ou=Groups,dc=example,dc=com": "operator"
      "cn=DevTeam,ou=Groups,dc=example,dc=com": "developer"
      "cn=Auditors,ou=Groups,dc=example,dc=com": "auditor"
      "cn=SRE_Lead,ou=Groups,dc=example,dc=com": "super_admin"

logs:
  storage_backend: "local"  # local, elasticsearch, loki
  level: "info"  # debug, info, warn, error
  retention_days: 30
  elasticsearch:
    url: "http://localhost:9200"
    index: "devops-logs"
    username: ""
    password: ""
  loki:
    url: "http://localhost:3100"
    tenant: ""

metrics:
  enabled: true
  path: "/metrics"

alerts:
  enabled: true
  rate_limit:
    window_seconds: 60
    max_alerts: 10
  slack:
    enabled: false
    default_channel: "#alerts"
  webhook:
    enabled: true
  email:
    enabled: false
    smtp_host: "smtp.example.com"
    smtp_port: 587
    from: "alerts@example.com"

physicalhost:
  ssh_pool_size: 10
  health_check_interval: 30s
  data_freshness_threshold: 30s
  ssh_timeout: 10s

k8s:
  default_kubeconfig_path: "~/.kube/config"

websocket:
  read_buffer_size: 1024
  write_buffer_size: 1024
  ping_interval: 30s
  ping_timeout: 5s
  max_message_size: 8192
  channels:
    - log
    - metric
    - device_event
    - pipeline_update
    - alert
```

---

## 6. Non-Functional Requirements

### 6.1 Performance

| Metric | Target |
|--------|--------|
| API Response Time (p95) | < 200ms |
| API Response Time (p99) | < 500ms |
| WebSocket Message Latency | < 50ms |
| Database Query Time (p95) | < 50ms |
| Concurrent Users | 100 |
| Concurrent WebSocket Connections | 1000 |

### 6.2 Reliability

| Metric | Target |
|--------|--------|
| Availability | 99.9% |
| Error Rate | < 0.1% |
| Recovery Time | < 5 min |
| Data Loss | 0% |

### 6.3 Scalability

| Resource | Limit |
|----------|-------|
| Devices | 10,000 |
| Pipelines | 1,000 |
| Pipeline Runs History | 100,000 |
| Log Entries | 100M (30-day retention) |
| Physical Hosts | 500 |

### 6.4 Security

| Requirement | Standard |
|-------------|----------|
| TLS Version | TLS 1.2+ |
| Password Hashing | bcrypt |
| JWT Algorithm | HS256 |
| Session Timeout | 24h |
| Password Min Length | 8 chars |
| Audit Retention | 1 year |

---

## 7. Frontend Components

### 7.1 Directory Structure

```
frontend/
├── public/
│   └── index.html
├── src/
│   ├── components/           # Reusable UI components
│   │   ├── common/          # Button, Input, Modal, Table, etc.
│   │   ├── layout/          # Header, Sidebar, Footer
│   │   └── charts/          # Metric charts, graphs
│   ├── pages/               # Route-level components
│   │   ├── Dashboard.tsx
│   │   ├── devices/
│   │   │   ├── DeviceList.tsx
│   │   │   ├── DeviceDetail.tsx
│   │   │   ├── DeviceGroups.tsx
│   │   │   └── DeviceActions.tsx
│   │   ├── pipelines/
│   │   │   ├── PipelineList.tsx
│   │   │   ├── PipelineEditor.tsx
│   │   │   └── PipelineRunDetail.tsx
│   │   ├── logs/
│   │   │   ├── LogViewer.tsx
│   │   │   └── LogFilters.tsx
│   │   ├── alerts/
│   │   │   ├── AlertChannels.tsx
│   │   │   ├── AlertHistory.tsx
│   │   │   └── AlertTrigger.tsx
│   │   ├── projects/
│   │   │   ├── BusinessLineList.tsx
│   │   │   ├── SystemDetail.tsx
│   │   │   ├── ProjectDetail.tsx
│   │   │   └── FinOpsReport.tsx
│   │   ├── hosts/
│   │   │   ├── HostList.tsx
│   │   │   └── HostMetrics.tsx
│   │   ├── k8s/
│   │   │   ├── ClusterList.tsx
│   │   │   └── WorkloadDashboard.tsx
│   │   ├── discovery/
│   │   │   └── DiscoveryScan.tsx
│   │   ├── audit/
│   │   │   └── AuditLogViewer.tsx
│   │   └── settings/
│   │       └── Settings.tsx
│   ├── hooks/               # Custom React hooks
│   │   ├── useApi.ts
│   │   ├── useWebSocket.ts
│   │   ├── useAuth.ts
│   │   └── usePagination.ts
│   ├── services/            # API clients
│   │   ├── api.ts           # Base API client
│   │   ├── auth.ts
│   │   ├── devices.ts
│   │   ├── pipelines.ts
│   │   ├── logs.ts
│   │   ├── alerts.ts
│   │   ├── projects.ts
│   │   ├── hosts.ts
│   │   └── k8s.ts
│   ├── contexts/            # React contexts
│   │   ├── AuthContext.tsx
│   │   ├── WebSocketContext.tsx
│   │   └── ThemeContext.tsx
│   ├── types/               # TypeScript types (mirrors backend)
│   │   ├── device.ts
│   │   ├── pipeline.ts
│   │   ├── log.ts
│   │   ├── alert.ts
│   │   └── project.ts
│   ├── utils/
│   │   ├── formatters.ts   # Date, number formatters
│   │   ├── validators.ts    # Form validation
│   │   └── constants.ts
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
└── package.json
```

### 7.2 Component Specifications

#### 7.2.1 Common Components

```typescript
// src/components/common/

// Button component
interface ButtonProps {
  variant: 'primary' | 'secondary' | 'danger' | 'ghost';
  size: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  loading?: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}

// Input component
interface InputProps {
  type: 'text' | 'password' | 'email' | 'number';
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  disabled?: boolean;
  label?: string;
}

// Table component
interface TableProps<T> {
  columns: Column<T>[];
  data: T[];
  loading?: boolean;
  pagination?: Pagination;
  onSort?: (field: keyof T, order: 'asc' | 'desc') => void;
  onRowClick?: (row: T) => void;
}

interface Column<T> {
  key: keyof T | string;
  header: string;
  width?: string;
  sortable?: boolean;
  render?: (value: any, row: T) => React.ReactNode;
}

// Modal component
interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  size?: 'sm' | 'md' | 'lg';
}

// Badge component
interface BadgeProps {
  variant: 'success' | 'warning' | 'error' | 'info' | 'neutral';
  children: React.ReactNode;
}

// StatusIndicator component
interface StatusIndicatorProps {
  status: 'online' | 'offline' | 'pending' | 'error';
  label?: string;
  pulse?: boolean;
}
```

#### 7.2.2 Layout Components

```typescript
// src/components/layout/

// Header component
interface HeaderProps {
  user: User;
  onLogout: () => void;
  onSettingsClick: () => void;
}

// Sidebar component
interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
  activePath: string;
}

// Navigation items
interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
  badge?: number;
  children?: NavItem[];
}

const navItems: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard', icon: <DashboardIcon /> },
  { path: '/devices', label: 'Devices', icon: <DeviceIcon /> },
  { path: '/pipelines', label: 'Pipelines', icon: <PipelineIcon /> },
  { path: '/logs', label: 'Logs', icon: <LogIcon /> },
  { path: '/alerts', label: 'Alerts', icon: <AlertIcon />, badge: unreadAlerts },
  { path: '/hosts', label: 'Hosts', icon: <HostIcon /> },
  { path: '/k8s', label: 'Kubernetes', icon: <K8sIcon /> },
  { path: '/projects', label: 'Projects', icon: <ProjectIcon /> },
  { path: '/discovery', label: 'Discovery', icon: <DiscoveryIcon /> },
  { path: '/audit', label: 'Audit Logs', icon: <AuditIcon /> },
];
```

#### 7.2.3 Page Components

```typescript
// src/pages/devices/DeviceList.tsx

interface DeviceListProps {
  // Filters from URL params
  filters: {
    state?: DeviceState;
    type?: DeviceType;
    search?: string;
    labels?: Record<string, string>;
  };
  pagination: {
    page: number;
    pageSize: number;
  };
}

interface DeviceRow {
  id: string;
  name: string;
  type: DeviceType;
  state: DeviceState;
  labels: Record<string, string>;
  lastSeen: string;
  actions: React.ReactNode;
}

// Device detail page
interface DeviceDetailProps {
  deviceId: string;
  tab: 'overview' | 'metrics' | 'config' | 'events' | 'actions';
}

// Pipeline editor
interface PipelineEditorProps {
  pipelineId?: string; // undefined for create
  onSave: (config: PipelineConfig) => void;
  onCancel: () => void;
}

// Log viewer with real-time streaming
interface LogViewerProps {
  filters: LogFilters;
  realtime?: boolean;
  onFilterChange: (filters: LogFilters) => void;
}
```

### 7.3 API Client Services

```typescript
// src/services/api.ts

class ApiClient {
  private baseURL: string;
  private token: string | null;

  constructor(baseURL: string) {
    this.baseURL = baseURL;
    this.token = localStorage.getItem('token');
  }

  setToken(token: string) {
    this.token = token;
    localStorage.setItem('token', token);
  }

  async get<T>(path: string, params?: Record<string, any>): Promise<T> {
    const url = new URL(`${this.baseURL}${path}`, window.location.origin);
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        if (v !== undefined) url.searchParams.set(k, String(v));
      });
    }
    const response = await fetch(url.toString(), {
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
      },
    });
    if (!response.ok) throw new ApiError(response);
    return response.json();
  }

  async post<T>(path: string, data?: any): Promise<T> {
    const response = await fetch(`${this.baseURL}${path}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
      },
      body: data ? JSON.stringify(data) : undefined,
    });
    if (!response.ok) throw new ApiError(response);
    return response.json();
  }

  // ... put, delete methods
}

// src/services/devices.ts
export const deviceService = {
  list: (params: ListParams) => api.get<DeviceListResponse>('/api/devices', params),
  get: (id: string) => api.get<Device>(`/api/devices/${id}`),
  create: (data: CreateDeviceRequest) => api.post<Device>('/api/devices', data),
  update: (id: string, data: UpdateDeviceRequest) => api.put<Device>(`/api/devices/${id}`, data),
  delete: (id: string) => api.delete(`/api/devices/${id}`),
  search: (query: string) => api.get<Device[]>(`/api/devices/search?search=${query}`),
  executeAction: (id: string, action: string, params?: object) =>
    api.post<ActionResult>(`/api/devices/${id}/actions`, { action, params }),
};
```

### 7.4 WebSocket Hook

```typescript
// src/hooks/useWebSocket.ts

interface UseWebSocketOptions {
  channels: WSChannel[];
  onMessage: (message: WSMessage) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  reconnectInterval?: number;
}

interface WSMessage {
  channel: WSChannel;
  type: string;
  data: any;
  timestamp: string;
}

function useWebSocket(options: UseWebSocketOptions) {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    const ws = new WebSocket(`${WS_BASE_URL}/ws`);

    ws.onopen = () => {
      setConnected(true);
      options.onConnect?.();
      // Subscribe to channels
      options.channels.forEach(channel => {
        ws.send(JSON.stringify({ action: 'subscribe', channel }));
      });
    };

    ws.onmessage = (event) => {
      const message: WSMessage = JSON.parse(event.data);
      options.onMessage(message);
    };

    ws.onclose = () => {
      setConnected(false);
      options.onDisconnect?.();
      // Reconnect after interval
      setTimeout(connect, options.reconnectInterval || 5000);
    };

    wsRef.current = ws;
  }, [options.channels]);

  useEffect(() => {
    connect();
    return () => wsRef.current?.close();
  }, [connect]);

  return { connected };
}
```

### 7.5 State Management

```typescript
// For global state, use React Context + useReducer
// For server state, use TanStack Query (React Query)

// Auth context
interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

type AuthAction =
  | { type: 'LOGIN_START' }
  | { type: 'LOGIN_SUCCESS'; payload: { user: User; token: string } }
  | { type: 'LOGIN_FAILURE' }
  | { type: 'LOGOUT' };

// Device list state (TanStack Query)
const { data, isLoading, error } = useQuery({
  queryKey: ['devices', filters, pagination],
  queryFn: () => deviceService.list({ ...filters, ...pagination }),
  staleTime: 30000, // 30 seconds
});
```

### 7.6 Routing

```typescript
// src/App.tsx

const routes = [
  { path: '/login', element: <LoginPage />, public: true },
  { path: '/dashboard', element: <DashboardPage /> },
  { path: '/devices', element: <DeviceListPage /> },
  { path: '/devices/:id', element: <DeviceDetailPage /> },
  { path: '/pipelines', element: <PipelineListPage /> },
  { path: '/pipelines/new', element: <PipelineEditorPage /> },
  { path: '/pipelines/:id', element: <PipelineDetailPage /> },
  { path: '/logs', element: <LogViewerPage /> },
  { path: '/alerts', element: <AlertChannelsPage /> },
  { path: '/alerts/history', element: <AlertHistoryPage /> },
  { path: '/projects', element: <BusinessLineListPage /> },
  { path: '/projects/:id/systems/:sysId', element: <SystemDetailPage /> },
  { path: '/projects/:id/systems/:sysId/projects/:projId', element: <ProjectDetailPage /> },
  { path: '/hosts', element: <HostListPage /> },
  { path: '/k8s', element: <ClusterListPage /> },
  { path: '/k8s/:name/workloads', element: <WorkloadDashboardPage /> },
  { path: '/discovery', element: <DiscoveryScanPage /> },
  { path: '/audit', element: <AuditLogPage /> },
  { path: '/settings', element: <SettingsPage /> },
];
```

---

## 8. E2E Test Scenarios

### 8.1 Device Management E2E

```typescript
// e2e/devices.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Device Management', () => {

  test('should list devices with pagination', async ({ page }) => {
    await page.goto('/devices');

    // Wait for table to load
    await expect(page.locator('table tbody tr')).toHaveCount(20); // default page size

    // Navigate to next page
    await page.click('button:has-text("Next")');
    await expect(page.locator('table tbody tr')).toHaveCount(20);
    await expect(page.locator('text=Showing 21-40')).toBeVisible();
  });

  test('should filter devices by state', async ({ page }) => {
    await page.goto('/devices?state=active');

    // URL should update
    await expect(page).toHaveURL(/state=active/);

    // All rows should show active state
    const states = page.locator('table tbody tr td:nth-child(3)');
    const count = await states.count();
    for (let i = 0; i < count; i++) {
      await expect(states.nth(i)).toHaveText('active');
    }
  });

  test('should search devices by name', async ({ page }) => {
    await page.goto('/devices');

    await page.fill('input[placeholder="Search devices..."]', 'web-server');
    await page.click('button:has-text("Search")');

    // Results should be filtered
    await expect(page.locator('table tbody tr td:nth-child(2)')).toContainText('web-server');
  });

  test('should create new device', async ({ page }) => {
    await page.goto('/devices');

    await page.click('button:has-text("Add Device")');
    await expect(page.locator('modal')).toBeVisible();

    await page.fill('input[name="name"]', 'test-device-001');
    await page.selectOption('select[name="type"]', 'physical_host');
    await page.fill('input[name="labels.env"]', 'test');
    await page.fill('input[name="labels.role"]', 'web');

    await page.click('button:has-text("Create")');

    // Should show success and navigate to device detail
    await expect(page.locator('text=Device created successfully')).toBeVisible();
    await expect(page).toHaveURL(/\/devices\/[a-f0-9-]+/);
  });

  test('should view device detail', async ({ page }) => {
    await page.goto('/devices');

    // Click first row
    await page.locator('table tbody tr').first().click();

    // Should navigate to detail page
    await expect(page).toHaveURL(/\/devices\/[a-f0-9-]+/);
    await expect(page.locator('h1')).toContainText('Device Details');
  });

  test('should execute device action', async ({ page }) => {
    await page.goto('/devices/123e4567-e89b-12d3-a456-426614174000');

    // Click actions dropdown
    await page.click('button:has-text("Actions")');
    await page.click('button:has-text("Restart")');

    // Confirmation dialog
    await expect(page.locator('modal')).toBeVisible();
    await expect(page.locator('text=Are you sure you want to restart this device?')).toBeVisible();

    await page.click('button:has-text("Confirm")');

    // Should show success message
    await expect(page.locator('text=Action completed successfully')).toBeVisible();
  });

  test('should transition device state', async ({ page }) => {
    await page.goto('/devices/123e4567-e89b-12d3-a456-426614174000');

    // Current state is pending
    await expect(page.locator('span:has-text("pending")')).toBeVisible();

    // Click register button
    await page.click('button:has-text("Register")');

    // State should transition to registered
    await expect(page.locator('span:has-text("registered")')).toBeVisible();
    await expect(page.locator('text=State transition: pending → registered')).toBeVisible();
  });
});
```

### 8.2 Pipeline E2E

```typescript
// e2e/pipelines.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Pipeline Management', () => {

  test('should create pipeline with YAML editor', async ({ page }) => {
    await page.goto('/pipelines/new');

    await page.fill('input[name="name"]', 'my-first-pipeline');
    await page.fill('textarea[name="description"]', 'Test pipeline');

    // YAML editor should have syntax highlighting
    const editor = page.locator('.yaml-editor');
    await expect(editor).toBeVisible();

    // Paste YAML
    await editor.fill(`
stages:
  - name: build
    commands:
      - echo "Building..."
  - name: test
    commands:
      - echo "Testing..."
strategy:
  type: rolling
  max_surge: 20
`);

    await page.click('button:has-text("Create Pipeline")');

    await expect(page.locator('text=Pipeline created successfully')).toBeVisible();
    await expect(page).toHaveURL(/\/pipelines\/[a-f0-9-]+/);
  });

  test('should execute pipeline and show real-time progress', async ({ page }) => {
    await page.goto('/pipelines/123e4567-e89b-12d3-a456-426614174000');

    await page.click('button:has-text("Execute")');

    // Should show running state
    await expect(page.locator('span:has-text("running")')).toBeVisible();

    // WebSocket should update stage progress
    await expect(page.locator('text=Stage: build')).toBeVisible();
    await expect(page.locator('.progress-bar')).toBeVisible();

    // After completion, should show success
    await expect(page.locator('span:has-text("success")')).toBeVisible({ timeout: 60000 });
  });

  test('should cancel running pipeline', async ({ page }) => {
    await page.goto('/pipelines/123e4567-e89b-12d3-a456-426614174000');

    // Start execution
    await page.click('button:has-text("Execute")');
    await expect(page.locator('span:has-text("running")')).toBeVisible();

    // Cancel
    await page.click('button:has-text("Cancel")');

    // Confirmation
    await page.click('button:has-text("Confirm")');

    await expect(page.locator('span:has-text("cancelled")')).toBeVisible();
  });

  test('should view pipeline run history', async ({ page }) => {
    await page.goto('/pipelines/123e4567-e89b-12d3-a456-426614174000');

    await page.click('button:has-text("View Runs")');

    // Should show run history table
    await expect(page.locator('table RunsHistory')).toBeVisible();

    // Should have at least one run
    await expect(page.locator('table tbody tr')).toHaveCount({ greaterThan: 0 });

    // Click first run to view details
    await page.locator('table tbody tr').first().click();
    await expect(page.locator('h2:has-text("Run Details")')).toBeVisible();
  });
});
```

### 8.3 Log Viewer E2E

```typescript
// e2e/logs.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Log Viewer', () => {

  test('should stream logs in real-time', async ({ page }) => {
    await page.goto('/logs');

    // Enable realtime toggle
    await page.click('button:has-text("Realtime")');

    // New logs should appear automatically via WebSocket
    const initialCount = await page.locator('.log-entry').count();

    // Wait for new log entry
    await page.waitForSelector('.log-entry', { state: 'attached' });

    const newCount = await page.locator('.log-entry').count();
    expect(newCount).toBeGreaterThan(initialCount);
  });

  test('should filter logs by level', async ({ page }) => {
    await page.goto('/logs');

    // Click error filter
    await page.click('button[title="Filter by error"]');

    // URL should update
    await expect(page).toHaveURL(/level=error/);

    // All visible logs should be error level
    const levels = page.locator('.log-entry .level');
    const count = await levels.count();
    for (let i = 0; i < Math.min(count, 10); i++) {
      await expect(levels.nth(i)).toHaveText('error');
    }
  });

  test('should search logs', async ({ page }) => {
    await page.goto('/logs');

    await page.fill('input[placeholder="Search logs..."]', 'connection refused');
    await page.click('button:has-text("Search")');

    // Results should match search term
    const messages = page.locator('.log-entry .message');
    const count = await messages.count();
    for (let i = 0; i < Math.min(count, 10); i++) {
      await expect(messages.nth(i)).toContainText('connection refused');
    }
  });

  test('should save and load filters', async ({ page }) => {
    await page.goto('/logs');

    // Set filters
    await page.click('button[title="Filter by error"]');
    await page.fill('input[name="search"]', 'timeout');

    // Save filter
    await page.click('button:has-text("Save Filter")');
    await page.fill('input[name="filterName"]', 'Errors with timeout');
    await page.click('button:has-text("Save")');

    // Should show success
    await expect(page.locator('text=Filter saved')).toBeVisible();

    // Load saved filter
    await page.click('button:has-text("My Filters")');
    await page.click('text=Errors with timeout');

    // Should restore filters
    await expect(page.locator('input[name="search"]')).toHaveValue('timeout');
  });
});
```

### 8.4 Alert E2E

```typescript
// e2e/alerts.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Alert Management', () => {

  test('should create Slack alert channel', async ({ page }) => {
    await page.goto('/alerts');

    await page.click('button:has-text("Add Channel")');
    await page.selectOption('select[name="type"]', 'slack');
    await page.fill('input[name="name"]', 'ops-slack');
    await page.fill('input[name="webhookUrl"]', 'https://hooks.slack.com/services/xxx');
    await page.fill('input[name="channel"]', '#ops-alerts');

    await page.click('button:has-text("Create")');

    await expect(page.locator('text=Channel created')).toBeVisible();
    await expect(page.locator('table tbody tr:has-text("ops-slack")')).toBeVisible();
  });

  test('should trigger alert and verify rate limiting', async ({ page }) => {
    await page.goto('/alerts');

    // Open trigger dialog
    await page.click('button:has-text("Trigger Alert")');

    await page.fill('input[name="name"]', 'test-alert');
    await page.selectOption('select[name="severity"]', 'warning');
    await page.fill('textarea[name="message"]', 'Test alert message');
    await page.selectOption('select[name="channel"]', 'ops-slack');

    await page.click('button:has-text("Trigger")');

    // Should show success
    await expect(page.locator('text=Alert triggered successfully')).toBeVisible();

    // Trigger same alert quickly
    await page.click('button:has-text("Trigger Alert")');
    await page.fill('input[name="name"]', 'test-alert');
    await page.selectOption('select[name="severity"]', 'warning');
    await page.click('button:has-text("Trigger")');

    // Should be rate limited
    await expect(page.locator('text=Alert rate limited')).toBeVisible();
    await expect(page.locator('text=Try again in 60 seconds')).toBeVisible();
  });

  test('should view alert history', async ({ page }) => {
    await page.goto('/alerts/history');

    // Should show history table
    await expect(page.locator('table')).toBeVisible();

    // Filter by severity
    await page.click('button:has-text("Critical")');
    await expect(page).toHaveURL(/severity=critical/);
  });
});
```

### 8.5 Authentication E2E

```typescript
// e2e/auth.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {

  test('should login with valid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[name="username"]', 'john.doe');
    await page.fill('input[name="password"]', 'secret123');

    await page.click('button:has-text("Sign In")');

    // Should redirect to dashboard
    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('text=Welcome, john.doe')).toBeVisible();
  });

  test('should reject invalid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[name="username"]', 'john.doe');
    await page.fill('input[name="password"]', 'wrongpassword');

    await page.click('button:has-text("Sign In")');

    await expect(page.locator('text=Invalid credentials')).toBeVisible();
    await expect(page).toHaveURL('/login');
  });

  test('should logout', async ({ page }) => {
    // Login first
    await page.goto('/login');
    await page.fill('input[name="username"]', 'john.doe');
    await page.fill('input[name="password"]', 'secret123');
    await page.click('button:has-text("Sign In")');
    await expect(page).toHaveURL('/dashboard');

    // Logout
    await page.click('button:has-text("john.doe")');
    await page.click('button:has-text("Logout")');

    await expect(page).toHaveURL('/login');
    await expect(page.locator('text=You have been logged out')).toBeVisible();
  });

  test('should redirect to login when unauthenticated', async ({ page }) => {
    await page.goto('/dashboard');

    await expect(page).toHaveURL('/login');
    await expect(page.locator('text=Please sign in')).toBeVisible();
  });
});
```

### 8.6 Project Hierarchy E2E

```typescript
// e2e/projects.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Project Hierarchy', () => {

  test('should create business line hierarchy', async ({ page }) => {
    await page.goto('/projects');

    // Create business line
    await page.click('button:has-text("Add Business Line")');
    await page.fill('input[name="name"]', 'E-Commerce');
    await page.fill('textarea[name="description"]', 'E-Commerce division');
    await page.click('button:has-text("Create")');

    await expect(page.locator('text=Business Line created')).toBeVisible();
    await expect(page.locator('text=E-Commerce')).toBeVisible();

    // Create system under business line
    await page.click('text=E-Commerce');
    await page.click('button:has-text("Add System")');
    await page.fill('input[name="name"]', 'Order System');
    await page.click('button:has-text("Create")');

    await expect(page.locator('text=System created')).toBeVisible();
    await expect(page.locator('text=Order System')).toBeVisible();

    // Create project under system
    await page.click('text=Order System');
    await page.click('button:has-text("Add Project")');
    await page.fill('input[name="name"]', 'order-backend');
    await page.selectOption('select[name="type"]', 'backend');
    await page.click('button:has-text("Create")');

    await expect(page.locator('text=Project created')).toBeVisible();
  });

  test('should link resource to project', async ({ page }) => {
    await page.goto('/projects');

    // Navigate to project
    await page.click('text=E-Commerce');
    await page.click('text=Order System');
    await page.click('text=order-backend');

    // Link device
    await page.click('button:has-text("Link Resource")');
    await page.selectOption('select[name="resourceType"]', 'device');
    await page.selectOption('select[name="resourceId"]', 'web-server-01');
    await page.click('button:has-text("Link")');

    await expect(page.locator('text=Resource linked')).toBeVisible();
    await expect(page.locator('table:has-text("web-server-01")')).toBeVisible();
  });

  test('should export FinOps report', async ({ page }) => {
    await page.goto('/projects');

    await page.click('button:has-text("Export FinOps Report")');

    // Should download CSV
    const download = await page.waitForEvent('download');
    expect(download.suggestedFilename()).toContain('finops');
    expect(download.suggestedFilename()).toContain('.csv');
  });

  test('should audit log visibility', async ({ page }) => {
    await page.goto('/audit');

    // Should see recent changes
    await expect(page.locator('table')).toBeVisible();

    // Filter by entity type
    await page.click('button:has-text("Project")');
    await expect(page).toHaveURL(/entity_type=project/);
  });
});
```

### 8.7 Physical Host E2E

```typescript
// e2e/hosts.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Physical Host Monitoring', () => {

  test('should list hosts with status', async ({ page }) => {
    await page.goto('/hosts');

    await expect(page.locator('table')).toBeVisible();

    // Should show status indicators
    await expect(page.locator('.status-indicator:has-text("online")')).toHaveCount({ greaterThan: 0 });
  });

  test('should view host metrics', async ({ page }) => {
    await page.goto('/hosts');

    await page.locator('table tbody tr').first().click();

    // Should show metrics charts
    await expect(page.locator('.cpu-chart')).toBeVisible();
    await expect(page.locator('.memory-chart')).toBeVisible();

    // Should show data freshness indicator
    await expect(page.locator('text=Fresh')).toBeVisible();
  });

  test('should trigger SSH heartbeat', async ({ page }) => {
    await page.goto('/hosts/123e4567-e89b-12d3-a456-426614174000');

    await page.click('button:has-text("Check Heartbeat")');

    // Should update heartbeat timestamp
    await expect(page.locator('text=Last heartbeat: just now')).toBeVisible();
  });

  test('should show stale data indicator when DB slow', async ({ page }) => {
    await page.goto('/hosts/123e4567-e89b-12d3-a456-426614174000');

    // When data is stale, should show warning
    const dataStatus = page.locator('.data-status');
    await expect(dataStatus).toBeVisible();

    const statusText = await dataStatus.textContent();
    if (statusText?.includes('Stale')) {
      await expect(page.locator('text=Data may be outdated')).toBeVisible();
    }
  });
});
```

### 8.8 Playwright Configuration

```typescript
// playwright.config.ts

import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',

  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
    },
  ],

  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
  },
});
```

---

## 9. Test Coverage Targets

| Module | Unit Test | Integration Test | E2E Test |
|--------|-----------|-----------------|----------|
| Device Management | 80% | 15 scenarios | 8 scenarios |
| Pipeline | 80% | 10 scenarios | 5 scenarios |
| Logging | 80% | 12 scenarios | 4 scenarios |
| Alerts | 80% | 10 scenarios | 4 scenarios |
| Authentication | 90% | 8 scenarios | 4 scenarios |
| RBAC | 80% | 10 scenarios | 6 scenarios |
| Project Hierarchy | 80% | 12 scenarios | 5 scenarios |
| Physical Hosts | 80% | 8 scenarios | 4 scenarios |
| K8s Management | 75% | 8 scenarios | 4 scenarios |
| Network Discovery | 75% | 6 scenarios | 3 scenarios |
| WebSocket | 80% | 6 scenarios | 4 scenarios |
| **Overall** | **80%** | **>100 scenarios** | **>50 scenarios** |
