# D 子项目 — UI 空白补全 + Middleware 链 CORS 设计

> **范围**:K8s pod log streaming UI (WS) + K8s pod exec UI (xterm.js + WS) + Service catalog on-call/runbook 写 UI + Audit log UI 增强 (scoped-Auditor + filters) + Middleware 链 CORS 前置
>
> **日期**: 2026-06-13
> **作者**: Claude
> **状态**: Design (待用户 review)
> **对应子项目**: D(5 子项目分解的第四个,接 B 之后)
> **总估时**: 1-1.5 周
> **前序**: B 子项目(commit `19fead83` v0.3.0.0)已落地 31 packages 全绿

---

## 1. 背景与动机

### 1.1 现状(v0.3.0.0)

| 现状 | 详情 |
|---|---|
| **K8s pod log 后端** | `internal/k8s/handler.go` `r.GET("/k8s/clusters/:id/namespaces/:ns/logs", logsP, h.GetLogs)` 已有 (one-shot)。WS stream endpoint 也在 `logstream/service.go` |
| **K8s pod exec 后端** | `r.POST("/k8s/clusters/:id/namespaces/:ns/pods/:pod/exec", execP, h.Exec)` 已有 |
| **K8sClusters.tsx** | 已有 pods 列表,`data-testid="pod-logs-button-${r.name}"` 按钮存在但无 panel 展开 |
| **Service catalog 后端** | POST/DELETE `/api/v1/services/:id/{oncall,runbook}` 已在 `servicecatalog/handler.go:87-94` |
| **Audit 后端** | `GET /api/v1/audit` 已支持 scoped-Auditor per-tenant 过滤 (B 子项目完成) |
| **Audit.tsx** | 已有基础 DataTable 显示 |
| **CORS middleware** | `internal/middleware/cors.go` 存在,`chain.go` 已包含 |

### 1.2 已知缺口(UI 层)

| 缺口 | 影响 | 估时 |
|---|---|---|
| K8s pod log streaming UI 缺失 | operator 看 pod 日志需 curl API,体验差 | 2 days |
| K8s pod exec UI 缺失 | "shell into pod" 是 K8s 用户最常用操作,无 UI 极大降级 | 2-3 days |
| Service catalog on-call/runbook 写 UI 缺失 | 排班/文档更新需手动 DB 操作 | 1 day |
| Audit log UI 缺 filters + scoped-Auditor 显式处理 | 审计查询难,几百个 event 找不到目标 | 1-2 days |
| CORS middleware 不在最前 | 跨 origin preflight OPTIONS 走不到 CORS,直接 401 | 1 day |

### 1.3 目标

5 项 UI/Middleware 空白全部补全,1-1.5 周完成,5 个 background agents 并行。

---

## 2. 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│          D 子项目 — UI 空白补全 + Middleware CORS                │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Middleware 链 (main.go, 主 session 协调):                       │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  旧: Recovery → RequestID → CORS → Logger → Auth+RBAC    │    │
│  │  新: CORS → Recovery → RequestID → Logger → Auth → RBAC  │    │
│  │  (CORS 移最前 + 全局 OPTIONS preflight 204)              │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Frontend (React):                                              │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  K8sClusters.tsx: pod row → "Logs" 按钮                  │    │
│  │   └─ <PodLogPanel clusterID pod namespace />             │    │
│  │       WS /api/v1/k8s/clusters/.../pods/.../logs/stream  │    │
│  │       auto-scroll + 过滤 + 颜色高亮                     │    │
│  │                                                          │    │
│  │  K8sClusters.tsx: pod row → "Exec" 按钮                  │    │
│  │   └─ <PodExecPanel clusterID pod namespace />             │    │
│  │       xterm.js + WS /pods/.../exec (bidirectional)       │    │
│  │                                                          │    │
│  │  Services.tsx: oncall/runbook 列表 → 写按钮              │    │
│  │   └─ <OnCallEditor> / <RunbookEditor> modals            │    │
│  │       POST/DELETE /api/v1/services/:id/{oncall,runbook} │    │
│  │                                                          │    │
│  │  Audit.tsx: 增强 filters                                │    │
│  │   └─ resource_type / timestamp range / actor_id search    │    │
│  │       ScopedAuditor: per-tenant 自动过滤                │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Backend (已就绪,无需新后端 endpoint):                          │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  K8s: GET /pods/.../logs/stream (WS)                     │    │
│  │  K8s: POST /pods/.../exec (bidirectional)               │    │
│  │  ServiceCatalog: POST/DELETE /services/:id/{oncall,runbook}│   │
│  │  Audit: GET /audit (已带 scoped-Auditor per-tenant)    │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### 关键设计原则

1. **CORS 移最前** — 让 preflight OPTIONS 在 Auth 前返 204
2. **WS panel 用 React Hooks** — 不引入 Redux 等新依赖
3. **xterm.js dynamic import** — 避免初始 bundle 变大
4. **Service catalog 写 UI** — 复用既有 POST/DELETE 后端
5. **Audit UI 增强** — 不重写,在现有 DataTable 上加 filter row
6. **CORS 配置** — `CORS_ALLOWED_ORIGINS` env var 注入,默认 `["*"]`

---

## 3. 关键 Component 详解

### 3.1 K8s pod log streaming UI (Agent 1)

**新组件** `frontend/src/components/k8s/PodLogPanel.tsx`:

```tsx
interface PodLogPanelProps {
  clusterID: string;
  namespace: string;
  podName: string;
  container?: string;
}

export function PodLogPanel(props: PodLogPanelProps) {
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filter, setFilter] = useState("");
  
  useEffect(() => {
    const params = props.container ? `?container=${props.container}` : "";
    const url = `/api/v1/k8s/clusters/${props.clusterID}/namespaces/${props.namespace}/pods/${props.podName}/logs/stream${params}`;
    const ws = new WebSocket(url);
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = (e) => setError(String(e));
    ws.onmessage = (evt) => {
      const line = JSON.parse(evt.data).line;
      setLines(prev => [...prev, line].slice(-1000)); // cap at 1000 lines
    };
    return () => ws.close();
  }, [props.clusterID, props.namespace, props.podName, props.container]);
  
  const filtered = filter
    ? lines.filter(l => l.includes(filter))
    : lines;
  
  // ... render: header (status) + controls + <pre> scrollable + colors
}
```

**集成点** `frontend/src/pages/K8sClusters.tsx`:
- 现有 `data-testid="pod-logs-button-${r.name}"` 按钮 — 接 onClick
- 弹 modal (复用 `frontend/src/components/common/Modal.tsx`)
- modal 内容: `<PodLogPanel ... />`

**单元测试** (vitest,新文件 `frontend/src/components/k8s/PodLogPanel.test.tsx`):
- mock `WebSocket` 全局
- 测:连接建立、收到消息 append `lines`、关闭清理

### 3.2 K8s pod exec UI (Agent 2)

**新依赖** `@xterm/xterm` + `@xterm/addon-fit`:
```json
{
  "dependencies": {
    "@xterm/xterm": "^5.5.0",
    "@xterm/addon-fit": "^0.10.0"
  }
}
```

**新组件** `frontend/src/components/k8s/PodExecPanel.tsx`:

```tsx
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export function PodExecPanel(props: PodExecPanelProps) {
  const divRef = useRef<HTMLDivElement>(null);
  const [term, setTerm] = useState<Terminal | null>(null);
  const [ws, setWs] = useState<WebSocket | null>(null);
  
  useEffect(() => {
    if (!divRef.current) return;
    const t = new Terminal({ cursorBlink: true, fontSize: 14 });
    const fit = new FitAddon();
    t.loadAddon(fit);
    t.open(divRef.current);
    fit.fit();
    setTerm(t);
    
    const url = `/api/v1/k8s/clusters/${props.clusterID}/namespaces/${props.namespace}/pods/${props.podName}/exec`;
    const sock = new WebSocket(url);
    sock.binaryType = "arraybuffer";
    sock.onopen = () => {
      sock.send(JSON.stringify({
        cmd: ["sh"],
        container: props.container || "",
      }));
    };
    sock.onmessage = (evt) => {
      const msg = JSON.parse(evt.data);
      if (msg.stdout) t.write(msg.stdout);
      if (msg.stderr) t.write(`\x1b[31m${msg.stderr}\x1b[0m`);
    };
    sock.onerror = () => t.write("\r\n\x1b[31mconnection failed\x1b[0m\r\n");
    t.onData((data) => {
      if (sock.readyState === WebSocket.OPEN) {
        sock.send(JSON.stringify({ stdin: data }));
      }
    });
    setWs(sock);
    
    return () => {
      sock.close();
      t.dispose();
    };
  }, [props.clusterID, props.namespace, props.podName]);
  
  return <div ref={divRef} style={{ width: '100%', height: '400px' }} />;
}
```

**集成点** 同 3.1,exec 按钮接 onClick → modal → `<PodExecPanel ... />`

**单元测试** (vitest,新文件):
- mock WebSocket + xterm
- 测:stdin 转发、stdout 写入 term、关闭清理

### 3.3 Service catalog on-call/runbook 写 UI (Agent 3)

**新组件** `frontend/src/components/service-catalog/OnCallEditor.tsx`:

```tsx
interface OnCallEditorProps {
  serviceID: string;
  onSaved: () => void;
}

export function OnCallEditor({ serviceID, onSaved }: OnCallEditorProps) {
  const [userID, setUserID] = useState("");
  const [shiftStart, setShiftStart] = useState("");
  const [shiftEnd, setShiftEnd] = useState("");
  const [timezone, setTimezone] = useState("UTC");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await apiPost(`/api/v1/services/${serviceID}/oncall`, {
        user_id: userID,
        shift_start: shiftStart,
        shift_end: shiftEnd,
        timezone,
      });
      onSaved();
    } catch (e) {
      setError(String(e));
    } finally {
      setSubmitting(false);
    }
  };
  
  // ... form rendering
}
```

**类似** `RunbookEditor.tsx`:
- 显示 runbook entries
- 写: `title`, `content` (markdown), `tags`

**集成点** `frontend/src/pages/Services.tsx`:
- 现有显示 on-call/runbook (read-only)
- 加 "Add on-call" / "Add runbook" 按钮 (Operator/SuperAdmin only)
- 加编辑/删除按钮 (带 confirm modal)

**单元测试** (vitest,新文件):
- 测:form 提交、optimistic update、删除 confirm

### 3.4 Audit log UI 增强 (Agent 4)

**修改** `frontend/src/pages/Audit.tsx`:

```tsx
export function Audit() {
  const [filters, setFilters] = useState<AuditFilters>({
    actor_id: "",
    resource_type: "",
    action: "",
    from: "",
    to: "",
  });
  const caller = useApi('/api/v1/auth/capabilities');  // 读 caller.Role
  const isScopedAuditor = caller?.role === 'ScopedAuditor';
  
  // URL 同步
  useEffect(() => {
    const params = qs.stringify(filters);
    window.history.replaceState(null, '', `?${params}`);
  }, [filters]);
  
  // ScopedAuditor 自动 inject project_id
  const queryFilters = isScopedAuditor
    ? { ...filters, project_id: caller.project_ids }
    : filters;
  
  // ... fetch /api/v1/audit?...
  // ... filter row UI: inputs + select + buttons
  // ... DataTable 显示
}
```

**测试**:
- 测:filter 提交带 query、URL 同步、ScopedAuditor 自动 inject

### 3.5 CORS 前置 (Agent 5 / 主 session 协调)

**修改** `cmd/devops-toolkit/main.go`:

```go
// 旧
r.Use(middleware.Chain(log, corsOrigins)...)
v1 := eng.Group("/api/v1", authMW.RequireAuth())

// 新
// CORS 必须最前,preflight 走通
cors := middleware.CORS(corsOrigins)
r.Use(cors)  // 应用到所有路由
r.OPTIONS("/*path", func(c *gin.Context) { c.Status(204) })  // preflight
r.Use(middleware.Recovery(log), middleware.RequestID(), middleware.Logger(log))
v1 := eng.Group("/api/v1", authMW.RequireAuth())
```

**修改** `internal/config/config.go`:

```go
type HTTPConfig struct {
    CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
    // ...
}
```

**env 加载**: `CORS_ALLOWED_ORIGINS` (逗号分隔),默认 `*`。

**测试**:
- `internal/middleware/cors_test.go` 加 preflight 测试
- `cmd/devops-toolkit/main_test.go` 加 OPTIONS 请求测试

---

## 4. 数据流与错误处理

### 4.1 关键数据流

**Flow 1: K8s pod log UI**

```
user clicks "Logs" on pod row
  ↓
<K8sClusters> opens <Modal>
  ↓
<PodLogPanel> mounts
  ↓
useEffect: new WebSocket(...)
  ↓ (WS open)
backend pushes log lines
  ↓
append to state.lines, auto-scroll if enabled
  ↓ (component unmount or WS close)
WS.close(), cleanup
```

**Flow 2: K8s pod exec UI**

```
user clicks "Exec" on pod row
  ↓
<PodExecPanel> mounts
  ↓
xterm.js Terminal created, fitted to div
  ↓
WS opens, sends {"cmd":["sh"], "container":"..."}
  ↓
backend connects to k8s apiserver exec, streams stdout/stderr
  ↓
xterm.onData (user keystroke) → ws.send({stdin: data})
  ↓
ws.onmessage ({stdout, stderr}) → term.write(data)
  ↓ (unmount)
term.dispose(), ws.close()
```

**Flow 3: Service catalog oncall 写**

```
user clicks "Add on-call" in <Services>
  ↓
<OnCallEditor> modal opens with form
  ↓
user fills user_id, shift_start, shift_end
  ↓
submit → POST /api/v1/services/S1/oncall
  ↓
on success: close modal, refetch service detail
on error: show error message in modal
```

**Flow 4: Audit log UI with scoped-Auditor**

```
user opens /audit (role=ScopedAuditor)
  ↓
<Audit> reads caller.Role from auth context
  ↓
ScopedAuditor: auto-inject project_id filter
  ↓
GET /api/v1/audit?project_id=X,Y,Z&...other filters
  ↓ (B 子项目 already returns per-tenant)
display filtered events
```

**Flow 5: CORS preflight**

```
Browser (cross-origin) sends OPTIONS /api/v1/foo
  ↓
CORS middleware (first in chain) intercepts
  ↓
sets Access-Control-Allow-Origin: <echoed>
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, X-User, X-User-Id
Access-Control-Max-Age: 86400
  ↓
returns 204
(Auth NOT triggered — preflight doesn't carry credentials)
```

### 4.2 错误处理约定

| 场景 | HTTP / UI 行为 | log |
|---|---|---|
| WS log connection fail | UI 显示红点 + "Reconnect" 按钮 | warn |
| WS log close unexpected | 自动重试 3 次,exponential backoff | warn |
| xterm exec WS fail | UI 显示 "Connection failed. Retry?" | error |
| xterm exec backend error | term.write("\r\n\x1b[31merror: " + msg + "\x1b[0m\r\n") | error |
| Service catalog 写 fail (网络) | modal 保留,显示 retry button | warn |
| Audit 写 fail (例如日期格式) | filter row 边框变红,tooltip 提示 | info |
| CORS preflight 拒绝 origin | 返 403,响应头有 `Access-Control-Allow-Origin` 缺失 | info |
| CORS main req 拒绝 origin | 浏览器 block,但 CORS middleware 不返 403(浏览器负责) | debug |

**关键约定**:
- **WS 错误绝不 throw** — 用 try/catch + state flag,UI 显示用户友好的"Retry"
- **CORS preflight 永不触发 Auth** — 浏览器 CORS spec 禁止 preflight 带 credentials
- **Audit filter 状态同步 URL** — refresh 页面不丢 filter

### 4.3 测试矩阵

| Component | Unit (vitest) | Integration (vitest + mock WS) | E2E (Playwright/manual) |
|---|---|---|---|
| `PodLogPanel` | ✅ mount/unmount | ✅ mock WS 收消息 | ⚠️ 文档化 |
| `PodExecPanel` | ✅ mount/unmount | ✅ mock WS + xterm | ⚠️ 文档化 |
| `OnCallEditor` | ✅ form 提交 | ⚠️ mock fetch | ⚠️ 文档化 |
| `RunbookEditor` | ✅ form 提交 | ⚠️ mock fetch | ⚠️ 文档化 |
| `Audit` (filters) | ✅ filter 状态 | ✅ URL 同步 | ⚠️ 文档化 |
| CORS preflight | ❌ (Go test) | ❌ (Go test in main_test.go) | ⚠️ 文档化 |
| 中间件链顺序 | ❌ (Go test) | ❌ | ⚠️ 文档化 |

**总测试新增**: ~25 vitest cases + ~5 Go cases
**E2E 集成**: ~3 Playwright cases(可后置,留 Phase 5)

### 4.4 风险 & 缓解

| 风险 | 缓解 |
|---|---|
| Agent 5 (CORS) 改 main.go 撞 Agent 1-4 依赖 | Agent 5 在主 session 协调下,最后 merge 阶段 commit |
| xterm.js 体积大 (~200KB) | 用 dynamic import,只在打开 exec panel 时加载 |
| WS 重连策略 — 频繁重连会刷屏 | 指数退避 1s/2s/4s,上限 3 次 |
| Service catalog POST 路由之前没测试覆盖 | Agent 3 加 vitest mock fetch 测试 |
| Audit filter 状态多 → URL 长 | 用 `qs.stringify` 压缩,只 sync 关键 filter |
| 5 个 agent 都可能撞 stop hallucination | 3-4 个并行,主 session 协调,1-2 个 stop 就自己手动做 |

---

## 5. 实施计划(5 phase,5 agent 并行)

### Phase 1-4: 4 个 agent 并行(frontend)
- **Agent 1** (`feat/d1-pod-log-ui`): K8s pod log UI (WS panel) — 2 days
- **Agent 2** (`feat/d2-pod-exec-ui`): K8s pod exec UI (xterm.js + WS) — 2-3 days
- **Agent 3** (`feat/d3-svc-catalog-write-ui`): Service catalog oncall/runbook 写 UI — 1 day
- **Agent 4** (`feat/d4-audit-ui-enhance`): Audit log UI 增强 — 1-2 days

### Phase 5: Agent 5 + 主 session 协调
- **Agent 5** (`feat/d5-cors-middleware`): CORS 前置 + preflight + config — 1 day
- 主 session: merge 协调 + 全量测试 + 文档

### 时间轴
- Day 0: spec 评审 + plan
- Day 1-3: Agent 1, 2, 3, 4 并行
- Day 4: Agent 5 + 主 session merge resolution
- Day 5: 主 session final test + 文档收尾

### 关键 commit 命名(预计 ~20 commits)

**Phase 1 (Agent 1, 1 commit)**
1. `feat(frontend): K8s pod log streaming panel (WS)`

**Phase 2 (Agent 2, 1 commit + 1 dep)**
2. `chore(frontend): install @xterm/xterm and @xterm/addon-fit`
3. `feat(frontend): K8s pod exec terminal (xterm.js + WS)`

**Phase 3 (Agent 3, 2 commits)**
4. `feat(frontend): Service catalog oncall write modal`
5. `feat(frontend): Service catalog runbook write modal`

**Phase 4 (Agent 4, 1 commit)**
6. `feat(frontend): Audit log filters + scoped-Auditor per-tenant`

**Phase 5 (Agent 5, 2 commits)**
7. `feat(config): add CORS_ALLOWED_ORIGINS config field`
8. `feat(middleware): CORS 前置 + preflight 204 + 完整 allow headers`

**Phase 5 主 session 收尾 (1 commit)**
9. `chore(release): v0.4.0.0 — D 子项目 UI 空白补全 + CORS 文档收尾`

### 每个 commit 的完成定义 (DoD)

- 该 commit 的 code 改动落地
- 单元测试新增/调整齐全,`pnpm test` 在该目录全绿
- 既有 17 frontend vitest 全绿
- 31 packages Go test 全绿
- `go vet ./...` warning 数 ≤ 1
- CHANGELOG.md 增一行
- commit message 用中文,符合 git log 风格

---

## 6. 风险与回退

### 风险

| 风险 | 缓解 |
|---|---|
| A 子项目改动面广(8+ directories),/loop agent 跑可能撞并发 | **分批 commit**: 每个 component 1-2 commit,/loop 起 4-5 agent 各管 1 项,主 session 协调 merge |
| xterm.js 体积大 | dynamic import |
| WS 重连策略 | 指数退避,3 次封顶 |
| CORS 改 main.go 撞其他 agent 依赖 | Agent 5 在主 session 协调下,最后 merge 阶段 commit |

### 回退(每个 phase 独立可回退)

| Phase | 回退方法 | 风险等级 |
|---|---|---|
| D1 K8s log UI | `git revert <merge>` (1 PR) | 🟢 低 |
| D2 K8s exec UI | `git revert` | 🟢 低 |
| D3 Service catalog 写 UI | `git revert` | 🟢 低 |
| D4 Audit log UI | `git revert` | 🟢 低 |
| D5 CORS 前置 | `git revert` (1 PR) | 🟡 中 — 影响跨 origin 客户端 |

---

## 7. 不做 (YAGNI)

- 不做 OpenAPI / Swagger UI 集成(留给后续)
- 不做前端 RBAC matrix 编辑器
- 不做 K8s exec 的 PTY resize(简单 terminal)
- 不做 Audit log 的导出 CSV/JSON(留 API + 客户端)
- 不做 Service catalog 的批量操作
- 不做 xterm.js 的 theme customization
- 不做 real-time K8s pod status 推送(已有 list)
- 不做 Grafana dashboard 嵌入(留 Phase C/D 后续)

---

## 8. 验收标准

### 完成定义 (Definition of Done)

D 子项目**完成**意味着:

1. **代码**: ~20 个 commit 全部在 main 上
2. **测试**: 25+ vitest 新增,5+ Go 新增,17 frontend vitest + 31 backend Go 全绿
3. **质量**: `go vet ./...` warning ≤ 1,`pnpm lint` 无 error
4. **文档**: CHANGELOG `[0.4.0.0]` section,DOCUMENT_INDEX 加 spec 索引
5. **功能**:
   - K8s pod log UI: 点 "Logs" 按钮 → modal → 实时日志滚动
   - K8s pod exec UI: 点 "Exec" → modal → xterm 终端,可输入命令
   - Service catalog oncall 写: 列表 → Add → 填表 → POST → 列表更新
   - Audit log filters: actor_id/resource_type/action/timestamp 范围;ScopedAuditor 自动 per-tenant
   - CORS: 跨 origin preflight 返 204 + 正确 headers,main req 跨 origin 走通

### 关键验收用例

| 验收项 | 验证 |
|---|---|
| 17 frontend vitest | `pnpm test` |
| 31 backend Go test | `go test -count=1 -p 1 -timeout 600s ./...` |
| vet | `go vet ./... 2>&1 \| wc -l` (≤ 1) |
| K8s log 实时 | 浏览器:点 Logs → 看到 K8s pod 日志流 |
| K8s exec 可用 | 浏览器:点 Exec → 输 `ls` → 看到输出 |
| Service catalog 写 | 浏览器:Add on-call → 提交 → 列表更新 |
| Audit filter | 浏览器:filter actor_id=alice → 看到 alice 的 events |
| ScopedAuditor 过滤 | ScopedAuditor login → /audit → 只看到 member 项目的 audit |
| CORS preflight | `curl -X OPTIONS -H "Origin: https://other.com" ...` 返 204 + CORS headers |

---

## 9. 未来子项目 Preview

D 完成后(预计 1-1.5 周):

**E 子项目** (1 周): 性能 + 容量基线
- k6 100/300/1000 VU 场景
- p95+p99+error rate 报告
- Go pprof 内存/协程 profile
- 31 packages + 17 vitest 全绿的性能基线

D 后续增强 (留后续子项目):
- K8s cluster 详情 UI / K8s pod YAML viewer
- WebSocket auth (token-based) — 当前 dev-bypass 模式
- 多 cluster 切换 UI
- Service catalog 的批量操作
- Audit log 的导出 CSV/JSON

D→E 总估时 2-3 周,完成 v0.3.0.0 → v0.4.0.0。

---

## 附录 A:文件清单(详细变更范围)

### 新增文件 (frontend)
```
frontend/src/components/k8s/PodLogPanel.tsx
frontend/src/components/k8s/PodLogPanel.test.tsx
frontend/src/components/k8s/PodExecPanel.tsx
frontend/src/components/k8s/PodExecPanel.test.tsx
frontend/src/components/service-catalog/OnCallEditor.tsx
frontend/src/components/service-catalog/OnCallEditor.test.tsx
frontend/src/components/service-catalog/RunbookEditor.tsx
frontend/src/components/service-catalog/RunbookEditor.test.tsx
```

### 修改文件 (frontend)
```
frontend/package.json                       (加 @xterm/xterm, @xterm/addon-fit)
frontend/src/pages/K8sClusters.tsx         (接 onClick → modal → Panel)
frontend/src/pages/Services.tsx             (加写按钮 + Editor)
frontend/src/pages/Audit.tsx               (加 filter row + URL sync)
```

### 修改文件 (backend)
```
cmd/devops-toolkit/main.go                 (CORS 前置, OPTIONS handler)
internal/config/config.go                  (CORSAllowedOrigins 字段 + env 加载)
internal/middleware/cors.go                (允许 GET/POST/PUT/DELETE/OPTIONS)
internal/middleware/cors_test.go           (preflight 测试)
cmd/devops-toolkit/main_test.go             (OPTIONS 请求测试)
```

### 跨 phase 依赖

```
D.Phase1 (K8s log UI) ──┐
D.Phase2 (K8s exec UI) ─┤
D.Phase3 (Svc catalog) ─┼──→ D.Phase5 (主 session 收尾 + 全量测试 + 文档)
D.Phase4 (Audit UI) ────┤
D.Phase5 (CORS) ────────┘
```

5 phase 可并行(除 Phase 5 收尾外)。CORS 改 main.go 可能被其他 agent 依赖 — Agent 5 在主 session 协调下最后 commit,避免冲突。
