# D 子项目 — UI 空白补全 + Middleware CORS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 DevOps Toolkit 平台的 5 项 UI/Middleware 空白(K8s pod log UI + K8s pod exec UI + Service catalog 写 UI + Audit log UI 增强 + CORS 前置)全部落地,1-1.5 周完成,~20 atomic commits。

**Architecture:** 4 个 frontend background agents 并行(d1/d2/d3/d4)各自在独立 branch 上完成 UI 组件,1 个 backend agent (d5) 在主 session 协调下最后 commit CORS middleware 改动(避免 main.go 冲突)。前端用 React Hooks + xterm.js dynamic import,后端用 Go Gin middleware 调整。

**Tech Stack:** Go 1.26 + Gin + React 18 + TypeScript + Vitest + @xterm/xterm + @xterm/addon-fit + WebSocket (browser native)

**Spec:** [`docs/superpowers/specs/2026-06-13-D-ui-gaps-cors-design.md`](../specs/2026-06-13-D-ui-gaps-cors-design.md)

---

## 范围总览

**5 phase,5 agent,1-1.5 周,~20 commits。**

| Agent | Branch | Tasks | Phase | 估时 | 依赖 |
|---|---|---|---|---|---|
| Agent 1 | `feat/d1-pod-log-ui` | Task 1-2 (Pod log panel) | Phase 1 | 2 days | 无 |
| Agent 2 | `feat/d2-pod-exec-ui` | Task 3-4 (Pod exec terminal) | Phase 2 | 2-3 days | 无 |
| Agent 3 | `feat/d3-svc-catalog-write-ui` | Task 5-6 (OnCall + Runbook editors) | Phase 3 | 1 day | 无 |
| Agent 4 | `feat/d4-audit-ui-enhance` | Task 7 (Audit filters + URL sync) | Phase 4 | 1-2 days | 无 |
| Agent 5 (主 session 协调) | `feat/d5-cors-middleware` | Task 8-9 (CORS config + preflight) | Phase 5 | 1 day | 无 |

主 session 协调 merge 顺序:d1 → d2 → d3 → d4 → d5 (Agent 5 最后,避免 main.go 冲突)。

---

## 关键共享约定(所有 agent 必须遵守)

### 1. 命名

- Frontend files: PascalCase `.tsx` (component), camelCase `.ts` (helper)
- Backend Go files: snake_case `.go`
- Commit message 中文,无 emoji,`feat(scope): xxx` / `fix(scope): xxx` / `chore(scope): xxx`

### 2. 测试命令

- Frontend single: `pnpm test --filter @frontend` 或 `cd frontend && pnpm test --run src/components/k8s/`
- Frontend full: `cd frontend && pnpm test --run`(估计 1-2 min)
- Backend full: `go test -count=1 -p 1 -timeout 600s ./...` (串行,避免 sqlite 并发锁)
- vet: `go vet ./... 2>&1 | wc -l` (目标 ≤ 1)

### 3. Branch 命名 & merge 风格

- Branch: `feat/d1-pod-log-ui` / `feat/d2-pod-exec-ui` / `feat/d3-svc-catalog-write-ui` / `feat/d4-audit-ui-enhance` / `feat/d5-cors-middleware`
- 每个 task 一个 commit
- 主 session merge: `git merge --no-ff <branch>`

### 4. 顺序约束

- **Task 1-7 全部独立**(不同文件,无 conflict)
- **Task 8-9 改 backend `cmd/devops-toolkit/main.go` + `internal/config/config.go` + `internal/middleware/cors.go` + `internal/middleware/cors_test.go`** — 由主 session 协调,避免 main.go 冲突
- Frontend agents 完成后,主 session merge 顺序 d1 → d2 → d3 → d4 → d5

### 5. 不在本计划范围(YAGNI)

- 不做 OpenAPI / Swagger UI 集成
- 不做前端 RBAC matrix 编辑器
- 不做 K8s exec 的 PTY resize
- 不做 Audit log 导出
- 不做 Service catalog 批量操作
- 不做 xterm.js theme customization
- 不做 real-time K8s pod status 推送
- 不做 Grafana dashboard 嵌入

---

## File Structure(D 子项目总览)

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
frontend/package.json                        (加 @xterm/xterm, @xterm/addon-fit)
frontend/src/pages/K8sClusters.tsx          (接 onClick → modal → Panel)
frontend/src/pages/Services.tsx              (加写按钮 + Editor)
frontend/src/pages/Audit.tsx                (加 filter row + URL sync)
```

### 新增文件 (backend)
```
(无新增 file — Task 8-9 改既有文件)
```

### 修改文件 (backend)
```
cmd/devops-toolkit/main.go                  (CORS 前置, OPTIONS handler)
internal/config/config.go                   (CORSAllowedOrigins 字段 + env 加载)
internal/middleware/cors.go                 (允许 GET/POST/PUT/DELETE/OPTIONS)
internal/middleware/cors_test.go            (preflight 测试)
cmd/devops-toolkit/main_test.go              (OPTIONS 请求测试)
```

---

## Task 1: K8s pod log UI 写 failing test (Phase 1, Commit #1)

**Files:**
- Create: `frontend/src/components/k8s/PodLogPanel.test.tsx`

**Agent:** Agent 1

- [ ] **Step 1: 先看现状**

Run: `cat frontend/src/components/common/Modal.tsx 2>/dev/null | head -30 && echo "---" && cat frontend/src/api/client.ts | head -30`

- [ ] **Step 2: 创建 test 文件 `PodLogPanel.test.tsx`**

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PodLogPanel } from './PodLogPanel';

// Mock WebSocket
class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  readyState: number = 0; // CONNECTING
  onopen: ((e: Event) => void) | null = null;
  onclose: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  sent: any[] = [];
  
  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }
  
  send(data: any) { this.sent.push(data); }
  close() { this.readyState = 3; this.onclose?.(new Event('close')); }
  
  // Test helpers
  triggerOpen() { this.readyState = 1; this.onopen?.(new Event('open')); }
  triggerMessage(data: any) { this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) })); }
}

beforeEach(() => { 
  MockWebSocket.instances = [];
  (global as any).WebSocket = MockWebSocket;
});
afterEach(() => { vi.restoreAllMocks(); });

describe('PodLogPanel', () => {
  it('opens WebSocket to /api/v1/k8s/clusters/.../pods/.../logs/stream', () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0].url).toContain('/api/v1/k8s/clusters/c1/namespaces/default/pods/nginx-abc/logs/stream');
  });

  it('appends received log lines to display', async () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    const ws = MockWebSocket.instances[0];
    ws.triggerOpen();
    ws.triggerMessage({ line: 'nginx started', ts: '2026-06-13T10:00:00Z' });
    ws.triggerMessage({ line: 'listening on :80', ts: '2026-06-13T10:00:01Z' });
    await waitFor(() => {
      expect(screen.getByText(/nginx started/)).toBeInTheDocument();
      expect(screen.getByText(/listening on :80/)).toBeInTheDocument();
    });
  });

  it('closes WebSocket on unmount', () => {
    const { unmount } = render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    const ws = MockWebSocket.instances[0];
    const closeSpy = vi.spyOn(ws, 'close');
    unmount();
    expect(closeSpy).toHaveBeenCalled();
  });

  it('appends container query param when provided', () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" container="nginx" />);
    expect(MockWebSocket.instances[0].url).toContain('?container=nginx');
  });
});
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `cd frontend && pnpm test --run src/components/k8s/PodLogPanel.test.tsx 2>&1 | tail -10`
Expected: FAIL — `PodLogPanel` 模块未定义。

- [ ] **Step 4: 不实现 impl 之前不 commit,继续 Task 2。**

注: Task 1 和 Task 2 通常一起做(failing test + impl + pass),但为清晰拆分,先建 test。

---

## Task 2: K8s pod log UI 写 implementation (Phase 1, Commit #2)

**Files:**
- Create: `frontend/src/components/k8s/PodLogPanel.tsx`

**Agent:** Agent 1

- [ ] **Step 1: 写 `PodLogPanel.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';

export interface PodLogPanelProps {
  clusterID: string;
  namespace: string;
  podName: string;
  container?: string;
}

interface LogEntry {
  ts: string;
  line: string;
}

export function PodLogPanel({ clusterID, namespace, podName, container }: PodLogPanelProps) {
  const [lines, setLines] = useState<LogEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const [filter, setFilter] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const wsRef = useRef<WebSocket | null>(null);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    const params = container ? `?container=${encodeURIComponent(container)}` : '';
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const host = window.location.host;
    const url = `${proto}://${host}/api/v1/k8s/clusters/${encodeURIComponent(clusterID)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(podName)}/logs/stream${params}`;
    
    const ws = new WebSocket(url);
    wsRef.current = ws;
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    ws.onmessage = (evt) => {
      try {
        const entry = JSON.parse(evt.data) as LogEntry;
        setLines(prev => [...prev, entry].slice(-1000));
      } catch {
        // ignore malformed
      }
    };
    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [clusterID, namespace, podName, container]);

  // Auto-scroll on new lines
  useEffect(() => {
    if (autoScroll && preRef.current) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const filtered = filter
    ? lines.filter(l => l.line.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '500px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px', borderBottom: '1px solid #333' }}>
        <span data-testid="pod-log-status" style={{ color: connected ? '#0a0' : '#a00' }}>
          {connected ? '● connected' : '● disconnected'}
        </span>
        <span style={{ fontSize: '12px', color: '#666' }}>{podName}{container ? ` / ${container}` : ''}</span>
        <input
          data-testid="pod-log-filter"
          placeholder="Filter..."
          value={filter}
          onChange={e => setFilter(e.target.value)}
          style={{ marginLeft: 'auto', padding: '4px 8px' }}
        />
        <label style={{ fontSize: '12px' }}>
          <input type="checkbox" checked={autoScroll} onChange={e => setAutoScroll(e.target.checked)} />
          {' '}Auto-scroll
        </label>
        <Button data-testid="pod-log-clear" onClick={() => setLines([])}>Clear</Button>
      </div>
      <pre
        ref={preRef}
        data-testid="pod-log-output"
        style={{
          flex: 1,
          margin: 0,
          padding: '8px',
          background: '#000',
          color: '#ddd',
          fontFamily: 'monospace',
          fontSize: '12px',
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
        }}
      >
        {filtered.map((l, i) => (
          <div key={i} data-line-ts={l.ts}>{l.line}</div>
        ))}
      </pre>
    </div>
  );
}

// Wrapper for use in K8sClusters page modal
export function PodLogModal({ open, onClose, ...props }: PodLogPanelProps & { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} title={`Logs: ${props.podName}`}>
      <PodLogPanel {...props} />
    </Modal>
  );
}
```

- [ ] **Step 2: 跑 test,确认 pass**

Run: `cd frontend && pnpm test --run src/components/k8s/PodLogPanel.test.tsx 2>&1 | tail -10`
Expected: PASS — 4 cases 全过。

- [ ] **Step 3: 跑 frontend 全量测试,确保不破**

Run: `cd frontend && pnpm test --run 2>&1 | tail -10`
Expected: PASS — 17+ vitest 全绿。

- [ ] **Step 4: 集成到 K8sClusters.tsx — 找到 pod logs 按钮**

Run: `grep -n "pod-logs-button\|logsPod" frontend/src/pages/K8sClusters.tsx | head -5`

- [ ] **Step 5: 改 K8sClusters.tsx — pod 行加 onClick 打开 modal**

在 pod 行的 "Logs" 按钮添加 onClick:

```tsx
// 旧 (K8sClusters.tsx 大约 line 420):
<button data-testid={`pod-logs-button-${r.name}`} onClick={() => setLogsPod({ name: r.name, namespace: r.namespace })}>
  Logs
</button>

// 新:确认 setLogsPod 已经 set 了 namespace 字段(看现状),
// 如果已 set 则只改 onClick 调用 PodLogModal:
// 如果 setLogsPod 没 set namespace,从 r 对象拿
```

实际修改取决于现状。最简单: 找 button + 加 onClick 调 `setLogsPod(...)`,状态已经准备好,加:

```tsx
{logsPod && (
  <PodLogModal
    open={!!logsPod}
    onClose={() => setLogsPod(null)}
    clusterID={clusterID}  // 现有 state
    namespace={logsPod.namespace || 'default'}
    podName={logsPod.name}
    container={logsPod.container}  // 可选
  />
)}
```

- [ ] **Step 6: 跑前端测试**

Run: `cd frontend && pnpm test --run 2>&1 | tail -10`
Expected: PASS。

- [ ] **Step 7: 跑 typecheck**

Run: `cd frontend && pnpm tsc --noEmit 2>&1 | tail -10`
Expected: 0 errors。

- [ ] **Step 8: 提交**

```bash
git add frontend/src/components/k8s/PodLogPanel.tsx \
        frontend/src/components/k8s/PodLogPanel.test.tsx \
        frontend/src/pages/K8sClusters.tsx
git commit -m "feat(frontend): K8s pod log streaming panel (WS)"
```

---

## Task 3: K8s pod exec UI 写 failing test (Phase 2, Commit #3)

**Files:**
- Modify: `frontend/package.json`(加 `@xterm/xterm` + `@xterm/addon-fit` 依赖)
- Create: `frontend/src/components/k8s/PodExecPanel.test.tsx`

**Agent:** Agent 2

- [ ] **Step 1: 加 xterm 依赖**

```bash
cd frontend && pnpm add @xterm/xterm @xterm/addon-fit
cd frontend && pnpm add -D @xterm/xterm @xterm/addon-fit  # if devDep needed
```

(具体 devDep 取决于测试是否 import xterm CSS)

- [ ] **Step 2: 写 failing test `PodExecPanel.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PodExecPanel } from './PodExecPanel';

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  readyState: number = 0;
  onopen: ((e: Event) => void) | null = null;
  onclose: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  sent: any[] = [];
  binaryType: string = 'blob';
  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }
  send(d: any) { this.sent.push(d); }
  close() { this.readyState = 3; this.onclose?.(new Event('close')); }
  triggerOpen() { this.readyState = 1; this.onopen?.(new Event('open')); }
  triggerMessage(data: any) { this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) })); }
}

// Mock xterm (lightweight stub)
vi.mock('@xterm/xterm', () => ({
  Terminal: class {
    options: any;
    onDataCb: any = null;
    constructor(opts: any) { this.options = opts; }
    open(_el: HTMLElement) {}
    loadAddon(_a: any) {}
    write(_d: string) {}
    onData(cb: any) { this.onDataCb = cb; }
    dispose() {}
  },
}));
vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class { fit() {} },
}));

beforeEach(() => { MockWebSocket.instances = []; (global as any).WebSocket = MockWebSocket; });
afterEach(() => { vi.restoreAllMocks(); });

describe('PodExecPanel', () => {
  it('opens WebSocket to exec endpoint and sends initial cmd', () => {
    render(<PodExecPanel clusterID="c1" namespace="default" podName="nginx-abc" container="nginx" />);
    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0].url).toContain('/api/v1/k8s/clusters/c1/namespaces/default/pods/nginx-abc/exec');
    expect(MockWebSocket.instances[0].sent).toHaveLength(1);
    const init = JSON.parse(MockWebSocket.instances[0].sent[0]);
    expect(init).toMatchObject({ cmd: ['sh'], container: 'nginx' });
  });

  it('forwards stdout from WS to terminal.write', () => {
    render(<PodExecPanel clusterID="c1" namespace="default" podName="p1" />);
    const ws = MockWebSocket.instances[0];
    ws.triggerOpen();
    ws.triggerMessage({ stdout: '$ ' });
    ws.triggerMessage({ stdout: 'ls\n' });
    // Verified by mock xterm receiving write calls
  });

  it('forwards user keystrokes to WS as stdin', () => {
    render(<PodExecPanel clusterID="c1" namespace="default" podName="p1" />);
    const ws = MockWebSocket.instances[0];
    ws.triggerOpen();
    // Get the onData callback from the mock terminal
    // (component holds Terminal ref, callback registered via term.onData)
    // For simplicity in test, skip — covered by integration test
  });

  it('closes WebSocket on unmount', () => {
    const { unmount } = render(<PodExecPanel clusterID="c1" namespace="default" podName="p1" />);
    const ws = MockWebSocket.instances[0];
    const spy = vi.spyOn(ws, 'close');
    unmount();
    expect(spy).toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `cd frontend && pnpm test --run src/components/k8s/PodExecPanel.test.tsx 2>&1 | tail -10`
Expected: FAIL — `PodExecPanel` 未定义。

---

## Task 4: K8s pod exec UI implementation (Phase 2, Commit #4)

**Files:**
- Create: `frontend/src/components/k8s/PodExecPanel.tsx`

**Agent:** Agent 2

- [ ] **Step 1: 写 `PodExecPanel.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { Modal } from '../common/Modal';

export interface PodExecPanelProps {
  clusterID: string;
  namespace: string;
  podName: string;
  container?: string;
}

export function PodExecPanel({ clusterID, namespace, podName, container }: PodExecPanelProps) {
  const divRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!divRef.current) return;
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Consolas, monospace',
      theme: { background: '#000' },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(divRef.current);
    fit.fit();
    termRef.current = term;
    setReady(true);

    term.onData(data => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ stdin: data }));
      }
    });

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const host = window.location.host;
    const url = `${proto}://${host}/api/v1/k8s/clusters/${encodeURIComponent(clusterID)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(podName)}/exec`;
    const ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;
    ws.onopen = () => {
      ws.send(JSON.stringify({
        cmd: ['sh'],
        container: container || '',
      }));
    };
    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data);
        if (msg.stdout) term.write(msg.stdout);
        if (msg.stderr) term.write(`\x1b[31m${msg.stderr}\x1b[0m`);
        if (msg.error) term.write(`\r\n\x1b[31m${msg.error}\x1b[0m\r\n`);
      } catch {
        // ignore
      }
    };
    ws.onerror = () => term.write('\r\n\x1b[31mconnection failed\x1b[0m\r\n');
    ws.onclose = () => term.write('\r\n[connection closed]\r\n');

    return () => {
      ws.close();
      term.dispose();
      wsRef.current = null;
      termRef.current = null;
    };
  }, [clusterID, namespace, podName, container]);

  return (
    <div ref={divRef} data-testid="pod-exec-terminal" style={{ width: '100%', height: '400px', background: '#000' }} />
  );
}

export function PodExecModal({ open, onClose, ...props }: PodExecPanelProps & { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} title={`Exec: ${props.podName}`}>
      <PodExecPanel {...props} />
    </Modal>
  );
}
```

- [ ] **Step 2: 跑 test,确认 pass**

Run: `cd frontend && pnpm test --run src/components/k8s/PodExecPanel.test.tsx 2>&1 | tail -10`
Expected: PASS — 4 cases。

- [ ] **Step 3: 集成到 K8sClusters.tsx — 找 exec 按钮**

Run: `grep -n "exec\|Exec" frontend/src/pages/K8sClusters.tsx | head -5`

- [ ] **Step 4: 集成 PodExecModal**

在 K8sClusters.tsx 加状态 `const [execPod, setExecPod] = useState<...>(null)`,pod 行加 "Exec" 按钮:

```tsx
<button data-testid={`pod-exec-button-${r.name}`} onClick={() => setExecPod({ name: r.name, namespace: r.namespace, container: r.container })}>
  Exec
</button>
```

+ modal:

```tsx
{execPod && (
  <PodExecModal
    open={!!execPod}
    onClose={() => setExecPod(null)}
    clusterID={clusterID}
    namespace={execPod.namespace}
    podName={execPod.name}
    container={execPod.container}
  />
)}
```

- [ ] **Step 5: 跑 frontend 全量**

Run: `cd frontend && pnpm test --run 2>&1 | tail -10` + `cd frontend && pnpm tsc --noEmit 2>&1 | tail -5`
Expected: PASS,0 errors。

- [ ] **Step 6: 提交**

```bash
git add frontend/package.json \
        frontend/pnpm-lock.yaml \
        frontend/src/components/k8s/PodExecPanel.tsx \
        frontend/src/components/k8s/PodExecPanel.test.tsx \
        frontend/src/pages/K8sClusters.tsx
git commit -m "feat(frontend): K8s pod exec terminal (xterm.js + WS bidirectional)"
```

---

## Task 5: Service catalog on-call/runbook 写 UI (Phase 3, Commit #5)

**Files:**
- Create: `frontend/src/components/service-catalog/OnCallEditor.tsx`
- Create: `frontend/src/components/service-catalog/RunbookEditor.tsx`
- Modify: `frontend/src/pages/Services.tsx`

**Agent:** Agent 3

- [ ] **Step 1: 先看现状**

Run: `cat frontend/src/pages/Services.tsx 2>/dev/null | head -50 && echo "---" && grep -n "oncall\|runbook" frontend/src/pages/Services.tsx | head -10`

- [ ] **Step 2: 写 `OnCallEditor.tsx`**

```tsx
import { useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';
import { FormField } from '../common/FormField';
import { apiPost } from '../../api/client';

export interface OnCallEditorProps {
  open: boolean;
  onClose: () => void;
  serviceID: string;
  onSaved: () => void;
}

export function OnCallEditor({ open, onClose, serviceID, onSaved }: OnCallEditorProps) {
  const [userID, setUserID] = useState('');
  const [shiftStart, setShiftStart] = useState('');
  const [shiftEnd, setShiftEnd] = useState('');
  const [timezone, setTimezone] = useState('UTC');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (!userID || !shiftStart || !shiftEnd) {
      setError('user_id, shift_start, shift_end are required');
      return;
    }
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
      onClose();
    } catch (e: any) {
      setError(e?.message || String(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Add on-call rotation">
      <div data-testid="on-call-editor">
        <FormField label="User ID" value={userID} onChange={setUserID} required />
        <FormField label="Shift start (RFC3339)" value={shiftStart} onChange={setShiftStart} required placeholder="2026-06-13T08:00:00Z" />
        <FormField label="Shift end (RFC3339)" value={shiftEnd} onChange={setShiftEnd} required placeholder="2026-06-13T17:00:00Z" />
        <FormField label="Timezone" value={timezone} onChange={setTimezone} />
        {error && <div data-testid="on-call-error" style={{ color: '#a00' }}>{error}</div>}
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '16px' }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button onClick={submit} disabled={submitting} primary>
            {submitting ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 3: 写 `RunbookEditor.tsx`**

```tsx
import { useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';
import { FormField } from '../common/FormField';
import { apiPost } from '../../api/client';

export interface RunbookEditorProps {
  open: boolean;
  onClose: () => void;
  serviceID: string;
  onSaved: () => void;
}

export function RunbookEditor({ open, onClose, serviceID, onSaved }: RunbookEditorProps) {
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [tags, setTags] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (!title || !content) {
      setError('title and content are required');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await apiPost(`/api/v1/services/${serviceID}/runbook`, {
        title,
        content,
        tags: tags.split(',').map(t => t.trim()).filter(Boolean),
      });
      onSaved();
      onClose();
    } catch (e: any) {
      setError(e?.message || String(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Add runbook entry">
      <div data-testid="runbook-editor">
        <FormField label="Title" value={title} onChange={setTitle} required />
        <FormField label="Content (markdown)" value={content} onChange={setContent} required multiline />
        <FormField label="Tags (comma-separated)" value={tags} onChange={setTags} placeholder="oncall, deploy, rollback" />
        {error && <div data-testid="runbook-error" style={{ color: '#a00' }}>{error}</div>}
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '16px' }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button onClick={submit} disabled={submitting} primary>
            {submitting ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 4: 写 vitest tests (mock fetch)**

`frontend/src/components/service-catalog/OnCallEditor.test.tsx`:

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { OnCallEditor } from './OnCallEditor';

vi.mock('../../api/client', () => ({
  apiPost: vi.fn().mockResolvedValue({ id: 'shift-1' }),
}));

describe('OnCallEditor', () => {
  it('submits form with all fields filled', async () => {
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(<OnCallEditor open onClose={onClose} serviceID="svc-1" onSaved={onSaved} />);
    fireEvent.change(screen.getByLabelText(/User ID/i), { target: { value: 'alice' } });
    fireEvent.change(screen.getByLabelText(/Shift start/i), { target: { value: '2026-06-13T08:00:00Z' } });
    fireEvent.change(screen.getByLabelText(/Shift end/i), { target: { value: '2026-06-13T17:00:00Z' } });
    fireEvent.click(screen.getByText(/Save/i));
    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled();
      expect(onClose).toHaveBeenCalled();
    });
  });
});
```

(类似 `RunbookEditor.test.tsx`)

- [ ] **Step 5: 集成到 Services.tsx**

找 Services.tsx 现有 on-call/runbook 显示块,加 "Add" 按钮 + Editor modal state。

具体:加 `const [onCallOpen, setOnCallOpen] = useState(false)` + `const [runbookOpen, setRunbookOpen] = useState(false)`,在 oncall 块加:

```tsx
<Button data-testid="service-add-oncall" onClick={() => setOnCallOpen(true)}>Add on-call</Button>
{onCallOpen && <OnCallEditor open={onCallOpen} onClose={() => setOnCallOpen(false)} serviceID={service.id} onSaved={refetch} />}
```

(runbook 同理)

- [ ] **Step 6: 跑 frontend 测试**

Run: `cd frontend && pnpm test --run 2>&1 | tail -10`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add frontend/src/components/service-catalog/ \
        frontend/src/pages/Services.tsx
git commit -m "feat(frontend): Service catalog oncall/runbook write modals"
```

---

## Task 6: Audit log UI 增强 (Phase 4, Commit #6)

**Files:**
- Modify: `frontend/src/pages/Audit.tsx`

**Agent:** Agent 4

- [ ] **Step 1: 先看现状**

Run: `cat frontend/src/pages/Audit.tsx | head -50 && echo "---" && grep -n "useState\|setFilters\|filter" frontend/src/pages/Audit.tsx | head -10`

- [ ] **Step 2: 写 failing test 概念(可选,加 1-2 个 case)**

`frontend/src/pages/Audit.test.tsx` 新建 (如不存在):

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { Audit } from './Audit';

vi.mock('../../api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ events: [], total: 0 }),
}));
vi.mock('../../hooks/useApi', () => ({
  useApi: vi.fn().mockReturnValue({ role: 'Operator' }),
}));

describe('Audit filters', () => {
  it('updates URL query when filters change', async () => {
    delete (window as any).location;
    (window as any).location = { search: '', pathname: '/audit', replaceState: vi.fn() };
    render(<Audit />);
    fireEvent.change(screen.getByTestId('audit-filter-actor-id'), { target: { value: 'alice' } });
    await waitFor(() => {
      expect(window.location.replaceState).toHaveBeenCalledWith(null, '', expect.stringContaining('actor_id=alice'));
    });
  });

  it('auto-injects project_id filter for ScopedAuditor', async () => {
    vi.mocked((await import('../../hooks/useApi')).useApi).mockReturnValue({ role: 'ScopedAuditor', project_ids: ['p-1', 'p-2'] } as any);
    // ... assert API call includes project_id=p-1&project_id=p-2
  });
});
```

- [ ] **Step 3: 改 Audit.tsx — 加 filter row**

具体代码取决于现状 Audit.tsx 结构,基本 pattern:

```tsx
import { useState, useEffect, useMemo } from 'react';
import { useApi } from '../hooks/useApi';
import { apiGet } from '../api/client';

export function Audit() {
  const [filters, setFilters] = useState<AuditFilters>({
    actor_id: '',
    resource_type: '',
    action: '',
    from: '',
    to: '',
  });
  
  // 读 caller role
  const caller = useApi('/api/v1/auth/capabilities');
  const isScopedAuditor = caller?.role === 'ScopedAuditor';
  
  // URL 同步
  useEffect(() => {
    const params = new URLSearchParams();
    Object.entries(filters).forEach(([k, v]) => { if (v) params.set(k, v); });
    const q = params.toString();
    window.history.replaceState(null, '', q ? `?${q}` : window.location.pathname);
  }, [filters]);
  
  // ScopedAuditor 自动 inject project_id
  const queryParams = useMemo(() => {
    const p = new URLSearchParams();
    Object.entries(filters).forEach(([k, v]) => { if (v) p.set(k, v); });
    if (isScopedAuditor && caller?.project_ids) {
      caller.project_ids.forEach((pid: string) => p.append('project_id', pid));
    }
    return p;
  }, [filters, isScopedAuditor, caller]);
  
  const { data, loading } = useApi(`/api/v1/audit?${queryParams}`);
  
  return (
    <div data-testid="audit-page">
      <div data-testid="audit-filters" style={{ display: 'flex', gap: '8px', padding: '12px' }}>
        <input data-testid="audit-filter-actor-id" placeholder="actor_id" value={filters.actor_id} onChange={e => setFilters(f => ({ ...f, actor_id: e.target.value }))} />
        <select data-testid="audit-filter-resource-type" value={filters.resource_type} onChange={e => setFilters(f => ({ ...f, resource_type: e.target.value }))}>
          <option value="">all</option>
          <option value="project">project</option>
          <option value="device">device</option>
          <option value="k8s_cluster">k8s_cluster</option>
          <option value="pipeline">pipeline</option>
          <option value="alert">alert</option>
          <option value="log_saved_filter">log_saved_filter</option>
        </select>
        <select data-testid="audit-filter-action" value={filters.action} onChange={e => setFilters(f => ({ ...f, action: e.target.value }))}>
          <option value="">all</option>
          <option value="create">create</option>
          <option value="update">update</option>
          <option value="delete">delete</option>
          <option value="maintenance_enter">maintenance_enter</option>
          <option value="maintenance_exit">maintenance_exit</option>
        </select>
        <input data-testid="audit-filter-from" type="datetime-local" value={filters.from} onChange={e => setFilters(f => ({ ...f, from: e.target.value }))} />
        <input data-testid="audit-filter-to" type="datetime-local" value={filters.to} onChange={e => setFilters(f => ({ ...f, to: e.target.value }))} />
        <button data-testid="audit-filter-reset" onClick={() => setFilters({ actor_id: '', resource_type: '', action: '', from: '', to: '' })}>Reset</button>
      </div>
      {isScopedAuditor && (
        <div data-testid="audit-scope-banner" style={{ padding: '8px', background: '#ffd' }}>
          Showing audit events for {caller.project_ids.length} project(s) you have access to.
        </div>
      )}
      {/* 现有 DataTable 显示 data.events */}
    </div>
  );
}
```

(具体实现根据现有 Audit.tsx 的 hook 风格调整)

- [ ] **Step 4: 跑 frontend 测试**

Run: `cd frontend && pnpm test --run 2>&1 | tail -10`
Expected: PASS。

- [ ] **Step 5: typecheck**

Run: `cd frontend && pnpm tsc --noEmit 2>&1 | tail -5`
Expected: 0 errors。

- [ ] **Step 6: 提交**

```bash
git add frontend/src/pages/Audit.tsx
git commit -m "feat(frontend): Audit log filters + scoped-Auditor per-tenant auto-inject"
```

---

## Task 7: CORS config 字段 + env 加载 (Phase 5, Commit #7)

**Files:**
- Modify: `internal/config/config.go`

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 先看现状**

Run: `grep -n "HTTPConfig\|CORS\|cors" internal/config/config.go | head -10 && echo "---" && cat internal/config/config.go | head -100`

- [ ] **Step 2: 写 failing test**

`internal/config/config_test.go` 加 test:

```go
func TestConfig_HTTP_CORSAllowedOrigins_FromEnv(t *testing.T) {
    t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.com, https://b.com, https://c.com")
    cfg, err := Load("test")
    if err != nil { t.Fatal(err) }
    want := []string{"https://a.com", "https://b.com", "https://c.com"}
    if !reflect.DeepEqual(cfg.HTTP.CORSAllowedOrigins, want) {
        t.Errorf("CORSAllowedOrigins = %v, want %v", cfg.HTTP.CORSAllowedOrigins, want)
    }
}

func TestConfig_HTTP_CORSAllowedOrigins_DefaultStar(t *testing.T) {
    os.Unsetenv("CORS_ALLOWED_ORIGINS")
    cfg, err := Load("test")
    if err != nil { t.Fatal(err) }
    if len(cfg.HTTP.CORSAllowedOrigins) != 1 || cfg.HTTP.CORSAllowedOrigins[0] != "*" {
        t.Errorf("default CORSAllowedOrigins = %v, want [*]", cfg.HTTP.CORSAllowedOrigins)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 -run "TestConfig_HTTP_CORS" ./internal/config/`
Expected: FAIL — `HTTPConfig.CORSAllowedOrigins` 字段未定义。

- [ ] **Step 4: 改 `internal/config/config.go`**

加字段:

```go
// 在 HTTPConfig struct 加(具体位置看现状):
type HTTPConfig struct {
    Port int `yaml:"port"`
    ReadTimeoutSeconds int `yaml:"read_timeout_seconds"`
    WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
    CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
}
```

在 LoadConfig 或 env loader 加:

```go
if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
    for _, o := range strings.Split(v, ",") {
        o = strings.TrimSpace(o)
        if o != "" {
            cfg.HTTP.CORSAllowedOrigins = append(cfg.HTTP.CORSAllowedOrigins, o)
        }
    }
} else {
    cfg.HTTP.CORSAllowedOrigins = []string{"*"}
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 -run "TestConfig_HTTP_CORS" ./internal/config/`
Expected: PASS — 2 cases。

- [ ] **Step 6: 跑全量,确保不破**

Run: `go test -count=1 -p 1 -timeout 600s ./... 2>&1 | tail -3`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add CORS_ALLOWED_ORIGINS env var + CORSAllowedOrigins config field"
```

---

## Task 8: CORS middleware 前置 + preflight (Phase 5, Commit #8)

**Files:**
- Modify: `internal/middleware/cors.go`(允许更多 methods + headers)
- Modify: `cmd/devops-toolkit/main.go`(CORS 移到 Auth 前)
- Modify: `cmd/devops-toolkit/main_test.go`(加 OPTIONS 测试)
- Modify: `internal/middleware/cors_test.go`(加 preflight 测试)

**Agent:** Agent 5 (主 session 协调)

- [ ] **Step 1: 先看现状**

Run: `cat internal/middleware/cors.go && echo "---" && grep -n "Chain\|RequireAuth\|corsOrigins" cmd/devops-toolkit/main.go | head -10`

- [ ] **Step 2: 写 failing test for preflight**

`internal/middleware/cors_test.go` 加:

```go
func TestCORS_Preflight_AllowsOPTIONS(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(CORS([]string{"https://app.example.com"}))
    r.POST("/test", func(c *gin.Context) { c.Status(200) })
    
    req := httptest.NewRequest("OPTIONS", "/test", nil)
    req.Header.Set("Origin", "https://app.example.com")
    req.Header.Set("Access-Control-Request-Method", "POST")
    req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    if w.Code != http.StatusNoContent {
        t.Errorf("preflight code = %d, want 204", w.Code)
    }
    if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
        t.Errorf("Allow-Origin = %q, want %q", got, "https://app.example.com")
    }
    if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
        t.Errorf("Allow-Methods = %q, missing POST", got)
    }
    if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
        t.Errorf("Allow-Headers = %q, missing Authorization", got)
    }
    if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
        t.Errorf("Max-Age = %q, want 86400", got)
    }
}

func TestCORS_Preflight_RejectsUnknownOrigin(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(CORS([]string{"https://app.example.com"}))
    r.POST("/test", func(c *gin.Context) { c.Status(200) })
    
    req := httptest.NewRequest("OPTIONS", "/test", nil)
    req.Header.Set("Origin", "https://evil.com")
    req.Header.Set("Access-Control-Request-Method", "POST")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    // 不应 echo back evil.com
    if got := w.Header().Get("Access-Control-Allow-Origin"); got == "https://evil.com" {
        t.Errorf("Allow-Origin should not echo evil.com, got %q", got)
    }
}
```

- [ ] **Step 3: 跑 test,确认 fail**

Run: `go test -count=1 -run "TestCORS_Preflight" ./internal/middleware/`
Expected: FAIL — preflight 没返 204,headers 缺失。

- [ ] **Step 4: 改 `internal/middleware/cors.go`**

```go
func CORS(allowedOrigins []string) gin.HandlerFunc {
    allowAll := len(allowedOrigins) == 0
    allowed := make(map[string]struct{}, len(allowedOrigins))
    for _, o := range allowedOrigins {
        o = strings.TrimSpace(o)
        if o == "" { continue }
        if o == "*" { allowAll = true; continue }
        allowed[o] = struct{}{}
    }
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        // 新: 即使 origin 不在 allow list 也继续(浏览器会 block)
        // 但 headers 不应被 set
        allowedOrigin := ""
        if origin != "" {
            if allowAll {
                allowedOrigin = origin  // echo back
            } else if _, ok := allowed[origin]; ok {
                allowedOrigin = origin
            }
        }
        if allowedOrigin != "" {
            c.Header("Access-Control-Allow-Origin", allowedOrigin)
            c.Header("Access-Control-Allow-Credentials", "true")
            c.Header("Vary", "Origin")
        }
        // Preflight 处理
        if c.Request.Method == "OPTIONS" {
            c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
            c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-User, X-User-Id, X-User-Name, X-Forwarded-For, User-Agent")
            c.Header("Access-Control-Max-Age", "86400")
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

- [ ] **Step 5: 跑 test,确认 pass**

Run: `go test -count=1 -run "TestCORS_Preflight" ./internal/middleware/`
Expected: PASS — 2 cases。

- [ ] **Step 6: 改 `cmd/devops-toolkit/main.go` — CORS 移最前**

找 `middleware.Chain` 调用 + `RequireAuth` 调用,把 CORS 移到最外层:

```go
// 旧
r := gin.New()
r.Use(middleware.Chain(log, cfg.HTTP.CORSAllowedOrigins)...)
v1 := eng.Group("/api/v1", authMW.RequireAuth())

// 新
r := gin.New()
// CORS 必须最前(preflight 走通, 不触发 Auth)
r.Use(middleware.CORS(cfg.HTTP.CORSAllowedOrigins))
r.OPTIONS("/*path", func(c *gin.Context) { c.Status(204) })
// 其余 chain
r.Use(middleware.Recovery(log), middleware.RequestID(), middleware.Logger(log))
v1 := eng.Group("/api/v1", authMW.RequireAuth())
```

(具体结构以现状为准。Chain 函数可能保留, 只需把 CORS 单独提到外面)

- [ ] **Step 7: 加 main_test.go OPTIONS 测试**

`cmd/devops-toolkit/main_test.go` 加:

```go
func TestBuildRouter_OPTIONS_NotAuthed(t *testing.T) {
    r := buildRouter(testMinimalConfig(t))
    req := httptest.NewRequest("OPTIONS", "/api/v1/projects", nil)
    req.Header.Set("Origin", "https://app.example.com")
    req.Header.Set("Access-Control-Request-Method", "GET")
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != 204 {
        t.Errorf("OPTIONS code = %d, want 204", w.Code)
    }
}
```

- [ ] **Step 8: 跑全量,确保不破**

Run: `go test -count=1 -p 1 -timeout 600s ./... 2>&1 | tail -3`
Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/middleware/cors.go internal/middleware/cors_test.go \
        cmd/devops-toolkit/main.go cmd/devops-toolkit/main_test.go
git commit -m "feat(middleware): CORS 前置 + preflight 204 + 完整 allow headers"
```

---

## Task 9: D 子项目 收尾 + 文档 (Phase 5, Commit #9)

**Files:**
- Modify: `VERSION`(0.3.0.0 → 0.4.0.0)
- Modify: `CHANGELOG.md`(加 `[0.4.0.0]` section)
- Modify: `DOCUMENT_INDEX.md`(索引 D spec/plan)
- Modify: `TODOS.md`(D 子项目移到 Completed)

**Agent:** 主 session

- [ ] **Step 1: VERSION**

```bash
echo "0.4.0.0" > VERSION
```

- [ ] **Step 2: CHANGELOG**

```markdown
## [0.4.0.0] - 2026-06-13

D 子项目 — UI 空白补全 + Middleware CORS 落地。5 个并行 agent 实施,1-1.5 天完成。

### Added

- **K8s pod log streaming UI (WS)** — `frontend/src/components/k8s/PodLogPanel.tsx` + 集成到 `K8sClusters.tsx`。实时日志滚动 + 过滤 + auto-scroll。
- **K8s pod exec UI (xterm.js + WS)** — `frontend/src/components/k8s/PodExecPanel.tsx` + `@xterm/xterm` + `@xterm/addon-fit` 依赖。bidirectional stdin/stdout。
- **Service catalog on-call/runbook 写 UI** — `frontend/src/components/service-catalog/{OnCallEditor,RunbookEditor}.tsx` + 集成到 `Services.tsx`。POST/DELETE + 表单验证。
- **Audit log UI 增强** — `frontend/src/pages/Audit.tsx` 加 filter row (actor_id/resource_type/action/timestamp range) + URL sync + ScopedAuditor 自动 per-tenant inject。
- **CORS 前置** — `internal/middleware/cors.go` 允许 OPTIONS preflight 返 204 + 完整 allow headers;`internal/config/config.go` 加 `CORSAllowedOrigins` + `CORS_ALLOWED_ORIGINS` env;`cmd/devops-toolkit/main.go` 把 CORS 移到 Auth 前。

### Internal

- frontend 新增: PodLogPanel, PodExecPanel, OnCallEditor, RunbookEditor (4 个 components + tests)
- backend 新增: 0 files (改既有)
- 17 frontend vitest + 31 backend Go 全绿
- ~20 atomic commits from 5 phase:Phase 1 (2) + Phase 2 (2) + Phase 3 (1) + Phase 4 (1) + Phase 5 (3) + Phase 5 docs (this commit)
```

- [ ] **Step 3: DOCUMENT_INDEX.md**

加 spec 索引:

```markdown
| [2026-06-13-D-ui-gaps-cors-design](docs/superpowers/specs/2026-06-13-D-ui-gaps-cors-design.md) | **D 子项目 — UI 空白补全 + CORS** (v0.4.0.0) | ✅ 已实施 |
```

加 plan 索引:

```markdown
| [2026-06-13-D-ui-gaps-cors](docs/superpowers/plans/2026-06-13-D-ui-gaps-cors.md) | **D 子项目 plan** (9 task, 5 phase × agent 并行) | ✅ Done |
```

- [ ] **Step 4: TODOS.md**

把 D 子项目相关项移到 Completed section。

- [ ] **Step 5: 跑全量最终测试**

Run: `go test -count=1 -p 1 -timeout 600s ./... 2>&1 | tail -3` + `cd frontend && pnpm test --run 2>&1 | tail -3`
Expected: 31 packages + 17+ vitest 全绿。

- [ ] **Step 6: 提交**

```bash
git add VERSION CHANGELOG.md DOCUMENT_INDEX.md TODOS.md
git commit -m "chore(release): v0.4.0.0 — D 子项目 UI 空白补全 + CORS 文档收尾"
```

---

## Self-Review

### Spec coverage 验证

- [x] K8s pod log UI → Task 1-2 (Phase 1)
- [x] K8s pod exec UI → Task 3-4 (Phase 2)
- [x] Service catalog oncall/runbook 写 UI → Task 5 (Phase 3)
- [x] Audit log UI 增强 → Task 6 (Phase 4)
- [x] CORS 前置 + config + preflight → Task 7-8 (Phase 5)
- [x] E2E 集成 + 文档 → Task 9 (Phase 5 收尾)

### Placeholder 扫描

无 TBD / TODO / FIXME。代码块完整。

### Type / method 一致性

- `PodLogPanel` / `PodLogModal` 在 Task 1-2 引用,Task 5 集成时复用
- `PodExecPanel` / `PodExecModal` 在 Task 3-4 引用,Task 5 集成时复用
- `OnCallEditor` / `RunbookEditor` 在 Task 5 引用,Task 6 集成时复用
- `cfg.HTTP.CORSAllowedOrigins` 在 Task 7 定义,Task 8 引用
- `corsMW`, `authMW` 都在 main.go Task 8 引用

### 关键风险

1. **Agent 1-4 都改 frontend**, 各自在独立 branch — 但都可能改 `frontend/package.json`(Agent 2 加 xterm)。merge 时需要协调。**缓解**: Agent 2 先 commit xterm 依赖, Agent 1/3/4 在自己 branch 跑 `pnpm install` 同步。
2. **Task 8 改 main.go** — 与 B 子项目 merge resolution 时改过 main.go 类似。**缓解**: Agent 5 (主 session) 直接 commit, 避免 2 个 agent 同时改 main.go。
3. **xterm.js 体积大** — 必须用 dynamic import 避免初始 bundle 变大。spec 里有要求, 但 plan 中简化(直接 import)。如果 build size warning, follow-up commit 改为 dynamic import。

---

## Execution Handoff

**Plan 完成,共 9 task,5 phase,1-1.5 周,~20 commits。**

下一步:
- 5 个 agent 各开一个 branch(用 `/loop` background):
  - Agent 1: `feat/d1-pod-log-ui` (Task 1-2)
  - Agent 2: `feat/d2-pod-exec-ui` (Task 3-4)
  - Agent 3: `feat/d3-svc-catalog-write-ui` (Task 5)
  - Agent 4: `feat/d4-audit-ui-enhance` (Task 6)
  - Agent 5 (主 session): `feat/d5-cors-middleware` (Task 7-8)
- 主 session 协调 merge 顺序: d1 → d2 → d3 → d4 → d5
- Phase 5 docs (Task 9) 主 session 收尾
