## Context

当前 `devops-toolkit-full-impl` 中的 k8s-cluster-management 规范描述的是基于 k3d 的本地集群生命周期管理（创建/删除/列表）。但 PRD_ENGINEERING_SUPPLEMENT.md 的数据库 schema 已定义为 `kubeconfig TEXT NOT NULL` 直接存储 kubeconfig 内容。

实际使用场景：用户导入 kubeconfig 到系统，系统通过该 kubeconfig 连接远程 K8s 集群进行操作，而非管理本地 k3d 集群。

## Goals / Non-Goals

**Goals:**
- 通过 kubeconfig 连接和管理远程 K8s 集群
- 存储 kubeconfig 内容到数据库，按需加载
- 支持集群健康检查、工作负载查询、Pod 日志获取等运维操作
- 支持多集群统一管理

**Non-Goals:**
- 不管理 k3d 本地集群的创建和删除（k3d 仅作为开发环境工具）
- 不做集群内部的资源变更（Deployment 扩缩容等可后续扩展）
- 不处理 kubeconfig 的过期刷新（由用户自行更新 kubeconfig）

## Decisions

### 1. Kubeconfig 存储格式

**决定**: 存储完整的 kubeconfig YAML 内容（`kubeconfig TEXT NOT NULL`），而非文件路径。

**原因**:
- kubeconfig 可包含多个集群/用户/context，完整存储确保灵活性
- 直接存储内容而非路径，避免路径依赖和权限问题
- 数据库字段已有 TEXT 类型，无需额外 schema 变更

**替代方案考虑**:
- 存储文件路径 → 路径依赖环境，部署一致性差
- 仅存 cluster/server URL → 无法处理多集群认证信息

### 2. Kubernetes 客户端创建

**决定**: 使用 `kubernetes.NewForConfig` 从 kubeconfig 动态构建客户端。

**原因**:
- client-go 是官方标准库，无需引入额外依赖
- 支持 kubeconfig 中定义的所有认证方式（cert、token、OIDC 等）
- `clientcmd.BuildConfigFromFlags` 自动解析 kubeconfig YAML

```go
import "k8s.io/client-go/tools/clientcmd"

func NewClientFromKubeconfig(kubeconfigContent string) (*kubernetes.Clientset, error) {
    loader := clientcmd.NewClientConfigFromBytes([]byte(kubeconfigContent))
    config, err := loader.ClientConfig()
    if err != nil {
        return nil, fmt.Errorf("invalid kubeconfig: %w", err)
    }
    return kubernetes.NewForConfig(config)
}
```

### 3. 集群注册验证

**决定**: 集群注册时必须验证连通性，验证失败则拒绝注册。

**原因**:
- 防止用户导入无效 kubeconfig 导致后续操作失败
- 提前发现认证信息过期问题
- 验证时获取集群版本信息用于元数据记录

### 4. 敏感信息处理

**决定**: kubeconfig 内容在日志中脱敏显示，仅显示集群名称和 server URL。

**原因**:
- kubeconfig 可能包含证书、token 等敏感信息
- 日志脱敏是标准安全实践

## Risks / Trade-offs

[Risk] kubeconfig 中的证书/token 可能过期
→ Mitigation: 数据库记录 `last_validated_at`，定期重新验证；过期时更新 status 为 `error`

[Risk] 大量集群导致连接池耗尽
→ Mitigation: 按需创建客户端，不维护持久连接；使用 context timeout 限制请求时间

[Risk] kubeconfig 格式解析失败
→ Mitigation: 注册时用 clientcmd 验证格式；返回具体错误信息

## Migration Plan

1. **Phase 1**: 更新 `k8s-cluster-management/spec.md` 规范，从 k3d 改为 kubeconfig 模式
2. **Phase 2**: 重写 `internal/k8s/cluster_client.go`，移除 k3d 调用，改用 client-go
3. **Phase 3**: 更新 service/handler 层逻辑
4. **Phase 4**: 更新 tasks.md 任务清单

**Rollback**: 无破坏性变更，纯接口兼容调整

## Open Questions

1. 是否需要支持 kubeconfig 自动过期检测和告警？
2. 多集群批量操作（如同时获取所有集群状态）的并发策略？
