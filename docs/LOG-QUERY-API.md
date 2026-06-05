# 日志查询 API 兼容性设计

**版本:** 1.0
**状态:** 草案
**最后更新:** 2026-06-05
**基于:** [openspec/specs/log-aggregation/spec.md](../openspec/specs/log-aggregation/spec.md)

> **目的:** 解决日志模块在不同存储后端（Local / Elasticsearch / Loki）之间切换时，API 契约断裂的问题。
>
> 核心原则：**后端可换，客户端零改动**。

---

## 1. 问题

| 维度 | Local | Elasticsearch | Loki |
|------|-------|---------------|------|
| 查询语法 | 子串匹配 | Lucene | LogQL |
| 全文搜索 | ✅ 子串 | ✅ | ✅ |
| 正则 | ❌ | ✅ | ✅ |
| 结构化过滤 | ❌ | ✅ | ✅ |
| 聚合 | 简单 | 完整 | 部分 |
| 时间范围 | 无限制 | 无限制 | 30 天 |
| Live tail | ✅ | ✅ | ✅ |
| 字段索引 | 无 | 灵活 | label 限制 |

**问题：** 直接透传后端语法 → 切换后端 = 改 API 契约 = 破坏所有客户端（前端、第三方集成、监控脚本）。

---

## 2. 设计目标

1. **后端切换零客户端改动** — 切 ES 到 Loki，前端代码不变
2. **能力透明** — 客户端能感知当前后端支持什么
3. **优雅降级** — 不支持的能力有兜底，而不是失败
4. **可演进** — 加新后端、加新字段不破坏现有客户端
5. **错误可重试判断** — 标准错误码，不依赖后端

---

## 3. 架构

```
┌────────────────────────────────────────────────────┐
│  客户端 (Frontend / 第三方)                        │
│  - 用统一的 Query DSL 请求                          │
│  - 先调 /capabilities 知道能用啥                    │
│  - 看响应 meta 知道是否降级                        │
└────────────────────────────────────────────────────┘
                      ↕ HTTP API (稳定契约 /api/v1/logs)
┌────────────────────────────────────────────────────┐
│  API 适配层 (Go)                                   │
│  - 接收标准 LogQuery，校验，补充默认值             │
│  - 调 backend.Query(Query) → LogPage              │
│  - 错误码映射 (backend err → 标准 err)             │
│  - 加 meta 信息到响应                              │
└────────────────────────────────────────────────────┘
                      ↕ LogBackend interface (Go)
┌────────────────────────────────────────────────────┐
│  后端实现                                          │
│  - LocalBackend (子串匹配，内存扫)                 │
│  - ElasticsearchBackend (翻译成 Lucene)            │
│  - LokiBackend (翻译成 LogQL)                     │
│  - 未来: ClickHouseBackend / S3Backend / ...      │
└────────────────────────────────────────────────────┘
```

---

## 4. 通用 Query DSL

### 4.1 LogQuery 结构

```go
type LogQuery struct {
    // ===== 通用段 (Universal) - 所有后端必须支持 =====
    StartTime *time.Time `json:"start_time,omitempty"`   // 默认 24h 前
    EndTime   *time.Time `json:"end_time,omitempty"`     // 默认 now
    Level     []string   `json:"level,omitempty"`        // [info,warn,error,debug]
    Source    []string   `json:"source,omitempty"`       // [api,web,worker,database]
    Search    string     `json:"search,omitempty"`        // 子串匹配
    Limit     int        `json:"limit,omitempty"`         // 默认 50, max 见 capabilities
    Offset    int        `json:"offset,omitempty"`        // 分页
    OrderBy   string     `json:"order_by,omitempty"`     // "time:desc" | "time:asc"
    
    // ===== 高级段 (Advanced) - 客户端先看 capabilities 再用 =====
    Regex            string            `json:"regex,omitempty"`           // 正则
    Fields           map[string]string `json:"fields,omitempty"`          // 字段过滤
    StructuredQuery  string            `json:"structured_query,omitempty"`// Lucene/LogQL 原生
}

type LogEntry struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    Level     string            `json:"level"`
    Source    string            `json:"source"`
    Message   string            `json:"message"`
    Host      string            `json:"host,omitempty"`
    Labels    map[string]string `json:"labels,omitempty"`
    Fields    map[string]any    `json:"fields,omitempty"`
}

type LogPage struct {
    Data       []LogEntry
    Pagination Pagination
    Meta       QueryMeta
}

type Pagination struct {
    Total   int  `json:"total"`
    Limit   int  `json:"limit"`
    Offset  int  `json:"offset"`
    HasMore bool `json:"has_more"`
}

type QueryMeta struct {
    Backend             string            `json:"backend"`               // "local" | "elasticsearch" | "loki"
    QueryTranslated     bool              `json:"query_translated"`      // 是否被翻译/降级
    TranslationStrategy string            `json:"translation_strategy"`  // "native" | "fallback" | "client-side"
    DegradedFeatures    []string          `json:"degraded_features"`     // 被降级的特性列表
    AppliedDefaults     map[string]any    `json:"applied_defaults"`      // 应用的默认值
    AppliedTimeRange    *TimeRange        `json:"applied_time_range,omitempty"`
    Capabilities        CapabilitiesSummary `json:"capabilities"`
}
```

### 4.2 URL 形式

**简单查询用 query string：**
```
GET /api/v1/logs?level=error&source=api&search=failed&start_time=2026-04-01T00:00:00Z
```

**复杂查询用 POST + body：**
```
POST /api/v1/logs/search
Content-Type: application/json

{
  "start_time": "2026-04-01T00:00:00Z",
  "end_time": "2026-04-30T23:59:59Z",
  "level": ["error", "warn"],
  "search": "failed",
  "fields": {"env": "prod"},
  "limit": 100
}
```

---

## 5. Backend 能力声明

### 5.1 Capabilities 结构

```go
type Capabilities struct {
    Backend           string   `json:"backend"`
    Version           string   `json:"version"`
    FullTextSearch    bool     `json:"full_text_search"`     // 子串
    RegexSearch       bool     `json:"regex_search"`
    StructuredQuery   bool     `json:"structured_query"`     // Lucene/LogQL
    Aggregations      []string `json:"aggregations"`         // ["count", "histogram", "percentiles"]
    LiveTail          bool     `json:"live_tail"`
    MaxTimeRange      string   `json:"max_time_range"`       // ISO 8601 duration, "" = unlimited
    MaxPageSize       int      `json:"max_page_size"`
    QueryableFields   []string `json:"queryable_fields"`     // ["message", "level", "source", "host", "labels.env"]
    SupportsFieldFilter bool   `json:"supports_field_filter"`
    SupportsGroupBy   bool     `json:"supports_group_by"`
}
```

### 5.2 各后端能力矩阵

| 能力 | Local | Elasticsearch | Loki |
|------|-------|---------------|------|
| FullTextSearch | ✅ | ✅ | ✅ |
| RegexSearch | ❌ | ✅ | ✅ |
| StructuredQuery | ❌ | ✅ Lucene | ✅ LogQL |
| Aggregations | [count] | [count, histogram, percentiles, date_histogram] | [count, histogram] |
| LiveTail | ✅ | ✅ | ✅ |
| MaxTimeRange | "" (无限) | "" (无限) | "720h" (30 天) |
| MaxPageSize | 1000 | 10000 | 5000 |
| SupportsFieldFilter | ❌ | ✅ | ✅ |
| SupportsGroupBy | ❌ | ✅ | 部分 |

### 5.3 端点

```
GET /api/v1/logs/capabilities

Response 200:
{
  "backend": "loki",
  "version": "1.0.0",
  "capabilities": {
    "full_text_search": true,
    "regex_search": true,
    "structured_query": true,
    "aggregations": ["count", "histogram"],
    "live_tail": true,
    "max_time_range": "720h",
    "max_page_size": 5000,
    "queryable_fields": ["message", "level", "source", "host", "labels.env", "labels.service"],
    "supports_field_filter": true,
    "supports_group_by": true
  }
}
```

---

## 6. 降级策略

### 6.1 降级矩阵

| 客户端请求 | Local | ES | Loki |
|-----------|-------|----|----|
| `search="error"` | ✅ 子串 | ✅ Lucene | ✅ LogQL \|~ |
| `regex="err.*"` | ❌ 400 | ✅ | ✅ |
| `fields.env="prod"` | ❌ 400 | ✅ term | ✅ label filter |
| `time_range=60d` | ✅ | ✅ | ⚠️ 自动缩到 30d + meta.degraded |
| `limit=10000` | ⚠️ 截到 1000 + meta | ✅ | ⚠️ 截到 5000 + meta |
| `aggregations=percentile` | ❌ 400 | ✅ | ❌ 400 |
| `group_by=host` | ❌ 400 | ✅ | ⚠️ 用 query 多次 + 聚合 |

### 6.2 降级原则

1. **永远不在没通知的情况下改变结果**
2. **降级 → `meta.degraded_features` 必须列出**
3. **截断 → `meta.degraded_features` 列出 + `meta.applied_limit`**
4. **不支持 → 400 错误 + 错误码 + capabilities 链接**
5. **不允许 client-side 后处理** — 翻译发生在服务端

### 6.3 翻译策略标记

```json
"meta": {
  "query_translated": true,
  "translation_strategy": "fallback",
  "degraded_features": [
    "time_range: capped to 720h (requested 1440h)",
    "limit: capped to 5000 (requested 10000)"
  ]
}
```

`translation_strategy` 取值：
- `"native"` — 后端直接支持，未做翻译
- `"fallback"` — 后端部分支持，做了降级
- `"client-side"` — 在 API 层做了过滤（不推荐使用）

---

## 7. 错误码标准化

### 7.1 错误响应结构

```json
{
  "error": "UNSUPPORTED_FEATURE",
  "message": "Regex search is not supported by the current backend",
  "details": {
    "feature": "regex_search",
    "backend": "local",
    "alternative": "Use search field for substring matching"
  },
  "hint": "Check GET /api/v1/logs/capabilities for supported features",
  "docs_url": "/docs/log-aggregation#capabilities"
}
```

### 7.2 错误码列表

| Code | HTTP | 含义 | 触发场景 |
|------|------|------|---------|
| `INVALID_QUERY` | 400 | 查询参数非法 | 时间格式错、limit 负数 |
| `UNSUPPORTED_FEATURE` | 400 | 后端不支持的特性 | regex on Local, percentile on Loki |
| `TIME_RANGE_EXCEEDED` | 400 | 时间范围超限 | Loki 请求 60 天 |
| `QUERY_TIMEOUT` | 408 | 后端查询超时 | ES/Loki 慢查询 |
| `RESULT_TOO_LARGE` | 413 | 结果集过大 | 10M 行扫描 |
| `RATE_LIMITED` | 429 | 客户端被限流 | 短时间高频查询 |
| `BACKEND_UNAVAILABLE` | 503 | 后端不可用 | ES/Loki 宕机 |

### 7.3 后端错误映射

| 后端错误 | 映射为 |
|---------|--------|
| ES: `circuit_breaking_exception` | `RESULT_TOO_LARGE` |
| ES: `parse_exception` | `INVALID_QUERY` |
| ES: `index_not_found_exception` | `BACKEND_UNAVAILABLE` (软) |
| Loki: `query time range exceeds limit` | `TIME_RANGE_EXCEEDED` |
| Loki: `client.timeout` | `QUERY_TIMEOUT` |
| Loki: `server returned 502` | `BACKEND_UNAVAILABLE` |
| Local: 无错误 | — |

### 7.4 Retry-After 策略

- `BACKEND_UNAVAILABLE`: Retry-After: 30
- `RATE_LIMITED`: Retry-After: 60
- `QUERY_TIMEOUT`: 不重试，让客户端决定
- 其他: 不重试

---

## 8. API 版本化

### 8.1 规则

| 变更 | 处理 | 客户端影响 |
|------|------|----------|
| 新增可选参数 | 不升版本 | 无 |
| 新增响应字段 | 不升版本 | 无 |
| 新增错误码 | 不升版本 | 旧客户端按通用错误处理 |
| 改变字段语义 | 升 minor | 客户端可能误读 |
| 删除参数/字段 | 升 major `/api/v2/logs` | 旧版本继续工作 6 个月 |
| 改变 LogEntry 字段 | 升 major | 旧版本冻结 schema |

### 8.2 弃用流程

1. 响应头加 `Deprecation: true`
2. 响应头加 `Sunset: 2026-12-31` (6 个月后)
3. 错误码用 `DEPRECATED_PARAMETER` 提示
4. 文档中标记 ⚠️ Deprecated

### 8.3 多版本共存

```
/api/v1/logs          # 当前
/api/v1/logs/capabilities
/api/v2/logs          # 未来
/api/v2/logs/capabilities
```

---

## 9. 客户端使用模式

### 9.1 推荐流程

```typescript
// 1. 应用启动时获取能力
const caps = await fetch('/api/v1/logs/capabilities').then(r => r.json())

// 2. 缓存到 store
logStore.setCapabilities(caps)

// 3. UI 根据能力渲染
if (caps.regex_search) showRegexInput()
if (!caps.structured_query) hideAdvancedQueryBuilder()

// 4. 每次查询都检查 meta
const result = await fetch('/api/v1/logs?search=error').then(r => r.json())
if (result.meta.degraded_features.length > 0) {
  showWarningToast(`Query was degraded: ${result.meta.degraded_features.join(', ')}`)
}
```

### 9.2 错误处理模板

```typescript
async function queryLogs(q: LogQuery): Promise<LogPage> {
  const res = await fetch('/api/v1/logs/search', {
    method: 'POST',
    body: JSON.stringify(q)
  })
  
  if (!res.ok) {
    const err = await res.json()
    switch (err.error) {
      case 'UNSUPPORTED_FEATURE':
        // 隐藏相关 UI 控件
        return handleUnsupported(err)
      case 'TIME_RANGE_EXCEEDED':
        // 自动缩小范围重试
        return retryWithSmallerRange(err)
      case 'BACKEND_UNAVAILABLE':
        // 显示降级提示，可能切到只读
        return handleBackendDown(err)
      case 'RATE_LIMITED':
        // 退避后重试
        await sleep(parseInt(res.headers.get('Retry-After') || '60'))
        return queryLogs(q)
      default:
        throw err
    }
  }
  
  return res.json()
}
```

---

## 10. 关键不变式 (Invariants)

1. **LogEntry schema 永远不变** — 字段名/类型/含义固定
2. **所有后端都能跑 Universal 子集** — 这是 SLA
3. **能力查询永远先于高级查询** — 客户端应先调 /capabilities
4. **错误永远可重试判断** — 有标准错误码
5. **降级永远透明** — meta.degraded_features 必须有
6. **后端切换 = 0 客户端代码改动** — 这是核心设计目标
7. **不允许 client-side 后处理** — 所有翻译在 API 层

---

## 11. 实施检查清单

### 后端 (Go)

- [ ] 定义 `LogBackend` interface（Query / Stats / Capabilities / Health）
- [ ] 实现 `LocalBackend`（子串匹配 + 内存扫描）
- [ ] 实现 `ElasticsearchBackend`（Lucene 翻译）
- [ ] 实现 `LokiBackend`（LogQL 翻译）
- [ ] 实现 `Capabilities` 聚合器
- [ ] 实现 `TranslationStrategy` 决策器
- [ ] 实现标准错误码映射器
- [ ] 实现 `/api/v1/logs/capabilities` 端点
- [ ] 实现 `/api/v1/logs/search` POST 端点
- [ ] 添加 unit tests：每个后端的 universal 子集
- [ ] 添加 contract tests：三个后端返回相同 LogEntry 形状
- [ ] 添加降级测试：每个降级路径有 meta 标记

### 前端 (React + TypeScript)

- [ ] `useLogCapabilities` hook — 启动时获取并缓存
- [ ] `LogQueryBuilder` 组件 — 根据 capabilities 动态渲染控件
- [ ] `DegradedFeaturesWarning` 组件 — 显示 meta.degraded_features
- [ ] `LogErrorBoundary` — 按 error code 分支处理
- [ ] 替换硬编码的 query 参数为 LogQuery 结构

### 运维

- [ ] 监控 `meta.translation_strategy` 分布
- [ ] 监控各错误码的频率
- [ ] 文档：`LOG-QUERY-API.md` (本文档)
- [ ] 文档：客户端迁移指南
- [ ] Runbook：常见错误码的处置

---

## 12. 演进路线

| 阶段 | 内容 |
|------|------|
| **v1.0 (现在)** | Universal 段 + Capabilities + 标准错误码 |
| v1.1 | 加 `Saved Searches`（用户保存查询模板） |
| v1.2 | 加 `Streaming Subscriptions`（WebSocket-based live tail） |
| v2.0 | 引入 `LogQL` 作为可选结构化查询（向后兼容） |
| v2.1 | 加新后端 ClickHouse / S3 / ... |

---

## 13. 参考

- [openspec/specs/log-aggregation/spec.md](../openspec/specs/log-aggregation/spec.md) — 形式化规格
- [Elasticsearch Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html)
- [Loki LogQL](https://grafana.com/docs/loki/latest/logql/)
- [JSON:API Specification](https://jsonapi.org/) — 响应格式参考
