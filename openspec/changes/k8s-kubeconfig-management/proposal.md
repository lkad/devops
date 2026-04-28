## Why

当前 K8s 集群管理设计基于 k3d 本地集群，但实际使用场景是通过 kubeconfig 连接和管理远程 K8s 集群。kubeconfig 本身包含集群认证信息，应该直接存储在数据库中，按需加载使用。

## What Changes

- 将 `k8s_clusters` 表的 `kubeconfig` 字段从 k3d 配置改为标准 kubeconfig YAML 内容存储
- 移除 k3d 客户端相关代码，用 kubeconfig 解析和客户端工厂替代
- 新增通过 kubeconfig 验证集群连通性的能力
- 支持多集群统一管理，所有集群通过 kubeconfig 接入

## Capabilities

### New Capabilities

- `k8s-cluster-management`: 通过 kubeconfig 管理远程 K8s 集群，包含集群注册、健康检查、工作负载管理、指标采集、Pod 操作等

### Modified Capabilities

- `k8s-cluster-management`: 从 k3d 本地集群管理改为 kubeconfig 远程集群管理（**BREAKING**）

## Impact

- **internal/k8s/models.go**: Cluster 模型变更，移除 k3d 相关字段
- **internal/k8s/cluster_client.go**: 改用 client-go + kubeconfig 动态创建 Kubernetes 客户端
- **internal/k8s/service.go**: 集群操作逻辑重写
- **internal/k8s/handler.go**: API 路径保持兼容，但底层实现变更
- **Database**: `k8s_clusters.kubeconfig` 字段存储标准 kubeconfig YAML
- **影响模块**: device-management（K8s 作为设备类型）、cicd-pipeline（K8s 部署目标）
