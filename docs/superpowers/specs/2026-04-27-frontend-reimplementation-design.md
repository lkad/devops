# Frontend Reimplementation Design

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the DevOps Toolkit frontend as a full React SPA with TypeScript, matching the existing DESIGN.md specifications.

**Architecture:** Single-page application with React 18, Vite bundler, TanStack Query for server state, Zustand for client state, and Tailwind CSS + Headless UI for styling. Follows the dark-first industrial/ utilitarian aesthetic from DESIGN.md.

**Tech Stack:** React 18, Vite, TypeScript, React Router v6, TanStack Query, Zustand, Tailwind CSS, Headless UI, Lucide React, React Hook Form, Zod

---

## 1. Project Structure

```
frontend/
├── src/
│   ├── main.tsx                    # Entry point, renders App
│   ├── App.tsx                     # Root component with providers
│   ├── index.css                   # Tailwind imports + global styles
│   ├── api/
│   │   ├── client.ts              # Base fetch wrapper with auth
│   │   ├── endpoints/
│   │   │   ├── auth.ts            # Login, logout, me
│   │   │   ├── devices.ts         # Device CRUD
│   │   │   ├── pipelines.ts       # Pipeline CRUD
│   │   │   ├── logs.ts            # Log queries
│   │   │   ├── metrics.ts        # Metrics
│   │   │   ├── alerts.ts         # Alert channels/trigger
│   │   │   ├── physical-hosts.ts # Physical hosts
│   │   │   ├── kubernetes.ts      # K8s clusters
│   │   │   └── projects.ts        # Project hierarchy
│   │   └── websocket.ts           # WebSocket client
│   ├── components/
│   │   ├── ui/                   # Base components (Button, Input, Select, Card, Badge, etc.)
│   │   ├── layout/              # PageLayout, Sidebar, Header
│   │   └── shared/              # DataTable, StatusBadge, EmptyState, LoadingSpinner
│   ├── pages/
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx
│   │   ├── devices/
│   │   │   ├── DeviceList.tsx
│   │   │   ├── DeviceDetail.tsx
│   │   │   └── DeviceRegister.tsx
│   │   ├── pipelines/
│   │   │   ├── PipelineList.tsx
│   │   │   ├── PipelineDetail.tsx
│   │   │   └── PipelineCreate.tsx
│   │   ├── logs/
│   │   │   └── LogViewer.tsx
│   │   ├── monitoring/
│   │   │   └── MetricsDashboard.tsx
│   │   ├── alerts/
│   │   │   ├── AlertChannels.tsx
│   │   │   └── AlertHistory.tsx
│   │   ├── physical-hosts/
│   │   │   ├── HostList.tsx
│   │   │   └── HostDetail.tsx
│   │   ├── kubernetes/
│   │   │   ├── ClusterList.tsx
│   │   │   └── ClusterDetail.tsx
│   │   └── projects/
│   │       ├── ProjectList.tsx
│   │       └── ProjectDetail.tsx
│   ├── hooks/
│   │   ├── useAuth.ts            # Auth state hook
│   │   ├── usePermissions.ts    # Permission check hook
│   │   └── useWebSocket.ts      # WebSocket subscription hook
│   ├── stores/
│   │   ├── authStore.ts          # Zustand auth state (existing)
│   │   └── uiStore.ts           # UI state (sidebar, modals)
│   └── lib/
│       ├── utils.ts              # Formatting helpers
│       └── validation.ts          # Zod schemas
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

## 2. API Client

### 2.1 Base Fetch Client (`client.ts`)

```typescript
// Auto-attaches JWT from authStore, handles 401 redirect
const apiClient = {
  get: <T>(url: string) => fetchJson<T>(url, { method: 'GET' }),
  post: <T>(url: string, data?: unknown) => fetchJson<T>(url, { method: 'POST', body: data }),
  put: <T>(url: string, data?: unknown) => fetchJson<T>(url, { method: 'PUT', body: data }),
  delete: <T>(url: string) => fetchJson<T>(url, { method: 'DELETE' }),
}
```

### 2.2 Endpoint Functions

Each module has dedicated functions in `api/endpoints/`:
- `GET /api/devices` → `getDevices()`, `getDevice(id)`
- `GET /api/pipelines` → `getPipelines()`, `getPipeline(id)`
- `GET /api/logs` → `getLogs(options)`
- `GET /metrics` → `getMetrics()`
- `GET/POST /api/alerts/channels` → `getAlertChannels()`, `createAlertChannel()`
- `WS /ws` → WebSocket subscription

## 3. Components

### 3.1 Base UI Components

| Component | Description |
|-----------|-------------|
| Button | Variants: primary, secondary, ghost, danger. Sizes: sm, md, lg |
| Input | Text input with label, error state, helper text |
| Select | Dropdown with Headless UI Listbox |
| Card | Surface container with optional header, padding |
| Badge | Status indicators with semantic colors |
| Modal | Dialog with Headless UI Dialog |
| Table | DataTable with sortable columns, pagination |

### 3.2 Layout Components

| Component | Description |
|-----------|-------------|
| PageLayout | Sidebar + Header + Content wrapper |
| Sidebar | Navigation with icons, collapsible |
| Header | User menu, breadcrumb, notifications |

### 3.3 Shared Components

| Component | Description |
|-----------|-------------|
| StatusBadge | Colored dot + text for device states |
| EmptyState | Icon + message for empty lists |
| LoadingSpinner | Centered spinner for loading states |
| ErrorMessage | Red alert for error display |

## 4. Pages

### 4.1 Login Page
- Username/password form with validation
- Submit calls `POST /api/auth/login`
- Success stores JWT, redirects to dashboard
- Error displays validation message

### 4.2 Dashboard Page
- Stats cards: device count, active pipelines, alert count
- Recent activity feed
- Quick actions panel

### 4.3 Device Pages
- List: Table with search, filter by type/status, pagination
- Detail: Device info, metrics, config template, state history
- Register: Form to register new device

### 4.4 Pipeline Pages
- List: All pipelines with status indicators
- Detail: Stage execution, logs, trigger button
- Create: YAML editor with validation

### 4.5 Log Viewer
- Query form: level, source, search, time range
- Results table with virtual scrolling
- Real-time streaming via WebSocket

### 4.6 K8s Cluster Pages
- List: Cluster cards with environment color coding
- Detail: Nodes, pods, workloads per cluster

### 4.7 Alert Pages
- Channels: CRUD for webhook/slack/email
- History: Paginated alert log with severity filters

### 4.8 Physical Host Pages
- List: Host cards with status indicators
- Detail: SSH info, metrics from InfluxDB/Prometheus

### 4.9 Project Pages
- Hierarchy viewer: Business Line → System → Project
- Resource linking per project
- FinOps report export

## 5. Authentication Flow

```
1. User visits app → App.tsx checks authStore for token
2. If no token → redirect to /login
3. User submits credentials → POST /api/auth/login
4. Success → store token + user in authStore, redirect to dashboard
5. API calls auto-include Authorization: Bearer <token>
6. 401 response → clear auth store, redirect to login
```

## 6. Design System Compliance

All components MUST use colors from DESIGN.md:

| Token | Value | Usage |
|-------|-------|-------|
| Primary | `#22d3ee` | Active states, primary actions |
| Background | `#0c1220` | Page background |
| Surface | `#151d2e` | Card backgrounds |
| Surface Elevated | `#1c2940` | Hover states |
| Border | `#2a3a54` | Borders |
| Text Primary | `#f1f5f9` | Primary text |
| Text Secondary | `#94a3b8` | Labels |
| Success | `#22c55e` | Success states |
| Warning | `#f59e0b` | Warning states |
| Error | `#ef4444` | Error states |

Typography: Geist for UI, JetBrains Mono for data/code.

## 7. Environment Filtering (K8s)

Per DESIGN.md section on Multi-Cluster Kubernetes UI:

- Cluster cards show environment color coding
- Search input filters by name (case-insensitive)
- Environment dropdown filters by type (dev/test/uat/prod)
- Selected cluster highlighted with cyan border + glow

## 8. Implementation Order

1. Project scaffolding (Vite, dependencies)
2. Tailwind + design tokens setup
3. API client with auth
4. Base UI components (Button, Input, Card, Badge)
5. Layout components (Sidebar, PageLayout)
6. Auth flow (Login, protected routes)
7. TanStack Query setup + hooks
8. Dashboard page
9. Device pages
10. Pipeline pages
11. Log viewer
12. K8s cluster pages
13. Alert pages
14. Physical host pages
15. Project pages
16. WebSocket integration for real-time updates
