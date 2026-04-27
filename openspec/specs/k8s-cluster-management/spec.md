# k8s-cluster-management

## MODIFIED Requirements

### Requirement: Cluster Registration
The system SHALL register K8s clusters by storing kubeconfig content.

```go
type K8sCluster struct {
    ID        uuid.UUID     `json:"id"`
    Name      string        `json:"name"`
    Kubeconfig string      `json:"kubeconfig,omitempty"` // stored in DB
    Status    ClusterStatus `json:"status"`
    Version   string        `json:"version"` // e.g., "v1.28.0"
    CreatedAt time.Time     `json:"createdAt"`
    UpdatedAt time.Time     `json:"updatedAt"`
}
```

#### Scenario: Register cluster with valid kubeconfig
- **WHEN** user sends POST /api/k8s/clusters with name and kubeconfig
- **THEN** system validates kubeconfig connectivity
- **AND** stores kubeconfig in database
- **AND** returns cluster info with status "active"

#### Scenario: Register cluster with invalid kubeconfig
- **WHEN** user sends POST /api/k8s/clusters with invalid kubeconfig
- **THEN** system returns 400 Bad Request
- **AND** error message indicates validation failure

#### Scenario: Register cluster with duplicate name
- **WHEN** user sends POST /api/k8s/clusters with existing name
- **THEN** system returns 409 Conflict

### Requirement: Cluster Lifecycle
The system SHALL support listing, getting, and deleting registered clusters.

#### Scenario: List all clusters
- **WHEN** user requests GET /api/k8s/clusters
- **THEN** system returns all registered clusters with status

#### Scenario: Get cluster by ID
- **WHEN** user requests GET /api/k8s/clusters/:id
- **THEN** system returns cluster details including version

#### Scenario: Delete cluster
- **WHEN** user sends DELETE /api/k8s/clusters/:id
- **THEN** system removes cluster from database
- **AND** returns 204 No Content

### Requirement: Health Checks
The system SHALL perform health checks on registered clusters.

#### Scenario: Check cluster health
- **WHEN** user requests GET /api/k8s/clusters/:id/health
- **THEN** system connects using stored kubeconfig
- **AND** returns node count, ready status, and API server version

#### Scenario: Cluster unreachable
- **WHEN** cluster is unreachable during health check
- **THEN** system returns status "error"
- **AND** error message indicates connection failure

### Requirement: Node Listing
The system SHALL list nodes from registered clusters.

```go
type NodeHealth struct {
    Name     string `json:"name"`
    Status   string `json:"status"` // "Ready", "NotReady"
    CPU      string `json:"cpu"`    // "15%"
    Memory   string `json:"memory"` // "32%"
    Age      string `json:"age"`
}
```

#### Scenario: Get node list
- **WHEN** user requests GET /api/k8s/clusters/:id/nodes
- **THEN** system returns list of all nodes with health info

### Requirement: Workload Listing
The system SHALL list workloads (Deployments, StatefulSets) from namespaces.

```go
type Workload struct {
    Name         string `json:"name"`
    Namespace    string `json:"namespace"`
    Type         string `json:"type"` // "Deployment", "StatefulSet"
    Ready        string `json:"ready"` // "3/3"
    Age          string `json:"age"`
}
```

#### Scenario: Get workload list
- **WHEN** user requests GET /api/k8s/clusters/:id/workloads
- **THEN** system returns deployments and statefulsets from all namespaces

#### Scenario: Get workload list in specific namespace
- **WHEN** user requests GET /api/k8s/clusters/:id/namespaces/:ns/workloads
- **THEN** system returns workloads only from specified namespace

### Requirement: Pod Logs
The system SHALL retrieve pod logs from clusters.

#### Scenario: Get pod logs
- **WHEN** user requests GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs
- **THEN** system returns container logs from the pod

#### Scenario: Get previous container logs
- **WHEN** user requests GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs?previous=true
- **THEN** system returns previous container logs (for restart)

### Requirement: Pod Exec
The system SHALL execute commands in pods.

#### Scenario: Execute command in pod
- **WHEN** user sends POST /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec with command
- **THEN** system executes command in container
- **AND** returns output

```go
type ExecResult struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}
```

### Requirement: Metrics Collection
The system SHALL collect CPU and memory metrics from cluster nodes.

#### Scenario: Get node metrics
- **WHEN** user requests GET /api/k8s/clusters/:id/metrics
- **THEN** system returns CPU and memory usage for all nodes

### Requirement: Kubeconfig Security
Stored kubeconfig content SHALL NOT be exposed in API responses.

#### Scenario: Get cluster details
- **WHEN** user requests GET /api/k8s/clusters/:id
- **THEN** kubeconfig field is null or masked in response

### Requirement: Multi-Cluster Operations
The system SHALL support operations across multiple clusters.

#### Scenario: Broadcast to all clusters
- **WHEN** system broadcasts log/metric events
- **THEN** message is sent to all connected cluster subscribers via WebSocket

## REMOVED Requirements

### Requirement: k3d Cluster Lifecycle
**Reason**: Cluster management changed from k3d local clusters to kubeconfig-based remote cluster registration
**Migration**: Use POST /api/k8s/clusters with kubeconfig instead of k3d creation
