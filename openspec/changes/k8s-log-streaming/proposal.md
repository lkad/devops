## Why

当前 K8s 日志只能通过 REST API 获取静态日志内容，无法做到实时流式查看。需要从 K8s apiserver 通过 watch 接口获取 Pod 日志流，通过 WebSocket 推送给前端，并与现有日志管理系统（log-aggregation）集成，支持历史日志查询和关联分析。

## What Changes

- 新增 K8s Pod 日志实时流式获取能力（通过 K8s watch API）
- 通过 WebSocket hub 推送日志到前端客户端
- 日志存入 log_entries 表，source="container"，携带 cluster/pod/namespace 元数据
- 支持"查看更多"分页加载历史日志
- 与现有日志聚合系统集成，统一下游存储

## Capabilities

### New Capabilities

- `k8s-pod-log-streaming`: K8s Pod 日志实时流式获取、WebSocket 推送、历史日志分页查询

### Modified Capabilities

- `log-aggregation`: 新增 source="container" 类型日志，支持按 cluster/pod/namespace 过滤
- `websocket-hub`: 支持 "container_log" channel，接收 K8s 日志流并广播

## Impact

- **internal/k8s/log_streamer.go**: 新增，使用 client-go watch API 获取 Pod 日志流
- **internal/logs/models.go**: LogEntry 增加 container_metadata（cluster_id, pod, namespace, container）
- **internal/websocket/hub.go**: 支持 container_log channel 订阅
- **API**: GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/stream (WebSocket)
- **Database**: log_entries 表已有结构无需变更，通过 metadata JSONB 存储 K8s 上下文
