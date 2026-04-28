# Tasks: K8s Kubeconfig Management Implementation

## 1. Package: K8s Models

- [ ] 1.1 Update internal/k8s/models.go (add Version field, remove k3d refs)
- [ ] 1.2 Create K8sCluster struct with Kubeconfig, Status, Version fields
- [ ] 1.3 Create NodeHealth, Workload, ExecResult structs
- [ ] 1.4 Add ClusterStatus constants (Active, Inactive, Error)

## 2. Package: Kubeconfig Client

- [ ] 2.1 Create internal/k8s/kubeconfig_client.go
- [ ] 2.2 Implement NewClientFromKubeconfig(kubeconfig string) (*kubernetes.Clientset, error)
- [ ] 2.3 Implement ValidateKubeconfig(kubeconfig string) error
- [ ] 2.4 Implement GetClusterVersion(kubeconfig string) (string, error)

## 3. Package: Repository

- [ ] 3.1 Create internal/k8s/repository.go
- [ ] 3.2 Implement Create(ctx, cluster) error
- [ ] 3.3 Implement GetByID(ctx, id) (*K8sCluster, error)
- [ ] 3.4 Implement List(ctx) ([]*K8sCluster, error)
- [ ] 3.5 Implement Delete(ctx, id) error
- [ ] 3.6 Implement UpdateStatus(ctx, id, status) error

## 4. Package: Service

- [ ] 4.1 Create internal/k8s/service.go
- [ ] 4.2 Implement RegisterCluster(ctx, name, kubeconfig) (*K8sCluster, error)
- [ ] 4.3 Implement GetCluster(ctx, id) (*K8sCluster, error)
- [ ] 4.4 Implement ListClusters(ctx) ([]*K8sCluster, error)
- [ ] 4.5 Implement DeleteCluster(ctx, id) error
- [ ] 4.6 Implement HealthCheck(ctx, id) (*ClusterHealth, error)
- [ ] 4.7 Implement GetNodes(ctx, id) ([]*NodeHealth, error)
- [ ] 4.8 Implement GetWorkloads(ctx, id, namespace) ([]*Workload, error)
- [ ] 4.9 Implement GetPodLogs(ctx, id, ns, pod, previous) (string, error)
- [ ] 4.10 Implement ExecInPod(ctx, id, ns, pod, command) (*ExecResult, error)
- [ ] 4.11 Mask kubeconfig in logs (sensitive data)

## 5. Package: Handler

- [ ] 5.1 Create internal/k8s/handler.go
- [ ] 5.2 POST /api/k8s/clusters - RegisterCluster
- [ ] 5.3 GET /api/k8s/clusters - ListClusters
- [ ] 5.4 GET /api/k8s/clusters/:id - GetCluster
- [ ] 5.5 DELETE /api/k8s/clusters/:id - DeleteCluster
- [ ] 5.6 GET /api/k8s/clusters/:id/health - HealthCheck
- [ ] 5.7 GET /api/k8s/clusters/:id/nodes - GetNodes
- [ ] 5.8 GET /api/k8s/clusters/:id/workloads - GetWorkloads
- [ ] 5.9 GET /api/k8s/clusters/:id/namespaces/:ns/workloads - GetWorkloads (namespaced)
- [ ] 5.10 GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs - GetPodLogs
- [ ] 5.11 POST /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec - ExecInPod

## 6. Integration

- [ ] 6.1 Register k8s routes in cmd/devops-toolkit/main.go
- [ ] 6.2 Add RBAC permissions for k8s endpoints
- [ ] 6.3 Wire up service with repository
- [ ] 6.4 Update WebSocket channel subscription for k8s events

## 7. Testing

- [ ] 7.1 Write unit tests for kubeconfig_client.go
- [ ] 7.2 Write unit tests for service.go with mock repository
- [ ] 7.3 Write integration tests with real kubeconfig
