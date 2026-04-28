## Context

K8s Pod 日志需要两种能力：
1. **实时流式**：新日志产生时立即推送到前端，类似 `kubectl logs -f`
2. **历史查询**：跳转到现有日志管理平台查看历史日志

**之前设计错误**：我错误地设计了一个独立的 `internal/k8s/log_repository.go` 来单独存储 K8s 日志。但实际上应该**复用现有 log-aggregation 平台**，K8s 日志只是作为 source="container" 的日志来源之一。

## Goals / Non-Goals

**Goals:**
- 从 K8s apiserver 获取 Pod 日志流
- 通过 WebSocket 实时推送日志到前端
- 日志通过现有 logsService 存入 log_entries 表
- "查看历史"跳转到日志管理平台，带过滤条件

**Non-Goals:**
- 不创建独立的 K8s 日志存储
- 不实现日志搜索/分析（由 log-aggregation 提供）
- 不做日志分页查询（由日志管理平台提供）

## Corrected Design

### 1. 日志流获取 + 实时推送

```
┌─────────────┐     Watch     ┌─────────────┐    Broadcast    ┌─────────────┐
│ K8s API     │ ───────────▶ │ LogStreamer │ ───────────▶ │ WebSocket   │
│ (apiserver) │              │             │              │ Hub         │
└─────────────┘              └─────────────┘              └─────────────┘
                                    │
                                    │ StoreLogEntry()
                                    ▼
                            ┌─────────────┐
                            │ logsService │ (existing)
                            │ log_entries │ (existing table)
                            └─────────────┘
```

**关键**：`LogStreamer` 只负责：
1. 从 K8s 获取日志流
2. 实时广播到 WebSocket（用于实时查看）
3. 调用 `logsService.CreateLogEntry()` 持久化

### 2. 复用现有 logs 服务

```go
// internal/k8s/log_streamer.go
func (s *LogStreamer) onLogLine(line string) {
    entry := &logs.LogEntry{
        Level:    parseLogLevel(line),
        Message:  line,
        Source:   "container",
        Metadata: JSONMap{
            "cluster_id":  s.clusterID,
            "cluster_name": s.clusterName,
            "namespace":   s.namespace,
            "pod":         s.pod,
            "container":   s.container,
        },
        Timestamp: time.Now(),
    }
    s.logsService.CreateLogEntry(s.ctx, entry)  // 复用现有服务
}
```

### 3. 查看历史 → 跳转日志平台

前端"查看历史"按钮不是调用 K8s API，而是跳转到日志管理页面：

```
/logs?source=container&cluster_id={clusterId}&namespace={namespace}&pod={pod}
```

日志管理平台的 `GET /api/logs` 已支持 `source` 和 `metadata` 过滤。

### 4. WebSocket 消息格式

```json
{
    "channel": "container_log",
    "type": "container_log",
    "data": {
        "clusterId": "uuid",
        "clusterName": "prod-cluster",
        "namespace": "default",
        "pod": "nginx-abc123",
        "container": "nginx",
        "message": "2026-04-27 10:00:00 GET /health 200",
        "level": "info",
        "timestamp": "2026-04-27T10:00:00Z"
    },
    "timestamp": "2026-04-27T10:00:00Z"
}
```

## API 设计（精简）

| Endpoint | 功能 |
|----------|------|
| `GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/stream` | WebSocket 实时流（订阅 container_log） |
| `GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/link` | 返回日志管理平台跳转链接 |

## Risks / Trade-offs

[Risk] 日志量大导致 log_entries 压力
→ Mitigation: 由 log-aggregation 的 retention policy 管控；可按 namespace 配置不同保留策略

[Risk] logsService CreateLogEntry 成为瓶颈
→ Mitigation: 异步写入；批量插入优化
