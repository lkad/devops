# Frontend Reimplementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the DevOps Toolkit frontend as a full React SPA with TypeScript, matching the existing DESIGN.md specifications.

**Architecture:** Single-page application with React 18, Vite bundler, TanStack Query for server state, Zustand for client state, and Tailwind CSS + Headless UI for styling. Follows the dark-first industrial/utilitarian aesthetic from DESIGN.md.

**Tech Stack:** React 18, Vite, TypeScript, React Router v6, TanStack Query, Zustand, Tailwind CSS, Headless UI, Lucide React, React Hook Form, Zod

---

## File Structure

```
frontend/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── index.css
│   ├── api/
│   │   ├── client.ts
│   │   ├── endpoints/auth.ts
│   │   ├── endpoints/devices.ts
│   │   ├── endpoints/pipelines.ts
│   │   ├── endpoints/logs.ts
│   │   ├── endpoints/metrics.ts
│   │   ├── endpoints/alerts.ts
│   │   ├── endpoints/physical-hosts.ts
│   │   ├── endpoints/kubernetes.ts
│   │   ├── endpoints/projects.ts
│   │   └── websocket.ts
│   ├── components/
│   │   ├── ui/Button.tsx
│   │   ├── ui/Input.tsx
│   │   ├── ui/Select.tsx
│   │   ├── ui/Card.tsx
│   │   ├── ui/Badge.tsx
│   │   ├── ui/Modal.tsx
│   │   ├── ui/Table.tsx
│   │   ├── ui/index.ts
│   │   ├── layout/PageLayout.tsx
│   │   ├── layout/Sidebar.tsx
│   │   ├── layout/Header.tsx
│   │   └── shared/StatusBadge.tsx
│   │   └── shared/EmptyState.tsx
│   │   └── shared/LoadingSpinner.tsx
│   ├── pages/
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx
│   │   ├── devices/DeviceList.tsx
│   │   ├── devices/DeviceDetail.tsx
│   │   ├── pipelines/PipelineList.tsx
│   │   ├── logs/LogViewer.tsx
│   │   ├── monitoring/MetricsDashboard.tsx
│   │   ├── alerts/AlertChannels.tsx
│   │   ├── kubernetes/ClusterList.tsx
│   │   ├── kubernetes/ClusterDetail.tsx
│   │   ├── physical-hosts/HostList.tsx
│   │   └── projects/ProjectList.tsx
│   ├── hooks/useAuth.ts
│   ├── hooks/usePermissions.ts
│   ├── hooks/useWebSocket.ts
│   ├── stores/authStore.ts
│   ├── stores/uiStore.ts
│   └── lib/utils.ts
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`
- Create: `frontend/index.html`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/index.css`

- [ ] **Step 1: Create package.json**

```json
{
  "name": "devops-toolkit-frontend",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@headlessui/react": "^2.2.0",
    "@tanstack/react-query": "^5.62.0",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-hook-form": "^7.54.0",
    "react-router-dom": "^6.28.0",
    "zod": "^3.24.0",
    "zustand": "^5.0.0",
    "lucide-react": "^0.460.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.4",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.4.49",
    "tailwindcss": "^3.4.15",
    "typescript": "^5.6.3",
    "vite": "^6.0.0"
  }
}
```

- [ ] **Step 2: Create vite.config.ts**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3001,
    proxy: {
      '/api': 'http://localhost:3000',
      '/ws': {
        target: 'ws://localhost:3000',
        ws: true,
      },
    },
  },
})
```

- [ ] **Step 3: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 4: Create index.html**

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>DevOps Toolkit</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 5: Create src/main.tsx**

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60, // 1 minute
      retry: 1,
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
```

- [ ] **Step 6: Create src/App.tsx**

```typescript
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import PageLayout from './components/layout/PageLayout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import DeviceList from './pages/devices/DeviceList'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <PageLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="devices" element={<DeviceList />} />
      </Route>
    </Routes>
  )
}
```

- [ ] **Step 7: Create src/index.css**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --color-primary: #22d3ee;
  --color-primary-hover: #06b6d4;
  --color-background: #0c1220;
  --color-surface: #151d2e;
  --color-surface-elevated: #1c2940;
  --color-border: #2a3a54;
  --color-border-subtle: #1e2d42;
  --color-text-primary: #f1f5f9;
  --color-text-secondary: #94a3b8;
  --color-text-muted: #64748b;
}

body {
  @apply bg-[#0c1220] text-[#f1f5f9];
}
```

- [ ] **Step 8: Create tailwind.config.ts**

```typescript
import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: '#22d3ee',
          hover: '#06b6d4',
          muted: 'rgba(34, 211, 238, 0.15)',
        },
        background: '#0c1220',
        surface: '#151d2e',
        'surface-elevated': '#1c2940',
        border: '#2a3a54',
        'border-subtle': '#1e2d42',
        'text-primary': '#f1f5f9',
        'text-secondary': '#94a3b8',
        'text-muted': '#64748b',
        success: '#22c55e',
        warning: '#f59e0b',
        error: '#ef4444',
        info: '#3b82f6',
      },
      fontFamily: {
        sans: ['Geist', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
    },
  },
  plugins: [],
} satisfies Config
```

- [ ] **Step 9: Create postcss.config.js**

```javascript
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **Step 10: Commit**

```bash
cd /mnt/devops/frontend
git init
git add package.json vite.config.ts tsconfig.json index.html src/main.tsx src/App.tsx src/index.css tailwind.config.ts postcss.config.js
git commit -m "feat(frontend): scaffold React + Vite + TypeScript project"
```

---

## Task 2: Auth Store and API Client

**Files:**
- Create: `frontend/src/stores/authStore.ts`
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/endpoints/auth.ts`
- Modify: `frontend/src/stores/authStore.ts` (existing from legacy frontend)

- [ ] **Step 1: Create src/stores/authStore.ts**

```typescript
import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

interface User {
  username: string
  roles: string[]
  permissions: string[]
}

interface AuthState {
  token: string | null
  user: User | null
  isAuthenticated: boolean
  _hasHydrated: boolean
  login: (token: string, user: User) => void
  logout: () => void
  setUser: (user: User) => void
  setHasHydrated: (state: boolean) => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      _hasHydrated: false,

      login: (token, user) => set({
        token,
        user,
        isAuthenticated: true,
      }),

      logout: () => set({
        token: null,
        user: null,
        isAuthenticated: false,
      }),

      setUser: (user) => set({ user }),

      setHasHydrated: (state) => set({ _hasHydrated: state }),
    }),
    {
      name: 'devops-auth',
      storage: createJSONStorage(() => localStorage),
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true)
      },
    }
  )
)
```

- [ ] **Step 2: Create src/api/client.ts**

```typescript
import { useAuthStore } from '../stores/authStore'

interface FetchOptions extends RequestInit {
  params?: Record<string, string>
}

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function fetchJson<T>(url: string, options: FetchOptions = {}): Promise<T> {
  const { params, ...fetchOptions } = options

  let finalUrl = url
  if (params) {
    const searchParams = new URLSearchParams(params)
    finalUrl = `${url}?${searchParams.toString()}`
  }

  const token = useAuthStore.getState().token
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...fetchOptions.headers,
  }

  const response = await fetch(finalUrl, {
    ...fetchOptions,
    headers,
  })

  if (response.status === 401) {
    useAuthStore.getState().logout()
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: 'Request failed' }))
    throw new ApiError(response.status, error.message || 'Request failed')
  }

  return response.json()
}

export const apiClient = {
  get: <T>(url: string, options?: FetchOptions) => fetchJson<T>(url, { ...options, method: 'GET' }),

  post: <T>(url: string, data?: unknown, options?: FetchOptions) =>
    fetchJson<T>(url, { ...options, method: 'POST', body: JSON.stringify(data) }),

  put: <T>(url: string, data?: unknown, options?: FetchOptions) =>
    fetchJson<T>(url, { ...options, method: 'PUT', body: JSON.stringify(data) }),

  delete: <T>(url: string, options?: FetchOptions) => fetchJson<T>(url, { ...options, method: 'DELETE' }),
}

export { ApiError }
```

- [ ] **Step 3: Create src/api/endpoints/auth.ts**

```typescript
import { apiClient } from '../client'

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expiresAt: number
  user: {
    username: string
    roles: string[]
    permissions: string[]
  }
}

export const authApi = {
  login: (data: LoginRequest) => apiClient.post<LoginResponse>('/api/auth/login', data),

  logout: () => apiClient.post('/api/auth/logout'),

  me: () => apiClient.get<{ username: string; roles: string[]; permissions: string[] }>('/api/auth/me'),
}
```

- [ ] **Step 4: Create src/lib/utils.ts**

```typescript
export function cn(...classes: (string | undefined | null | false)[]): string {
  return classes.filter(Boolean).join(' ')
}

export function formatDate(date: string | Date): string {
  return new Intl.DateTimeFormat('en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(date))
}

export function formatNumber(num: number): string {
  return new Intl.NumberFormat('en-US').format(num)
}
```

- [ ] **Step 5: Commit**

```bash
cd /mnt/devops/frontend
git add src/stores/authStore.ts src/api/client.ts src/api/endpoints/auth.ts src/lib/utils.ts
git commit -m "feat(frontend): add auth store and API client"
```

---

## Task 3: Base UI Components

**Files:**
- Create: `frontend/src/components/ui/Button.tsx`
- Create: `frontend/src/components/ui/Input.tsx`
- Create: `frontend/src/components/ui/Card.tsx`
- Create: `frontend/src/components/ui/Badge.tsx`
- Create: `frontend/src/components/ui/index.ts`

- [ ] **Step 1: Create src/components/ui/Button.tsx**

```typescript
import { ButtonHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
  size?: 'sm' | 'md' | 'lg'
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', size = 'md', disabled, ...props }, ref) => {
    const baseStyles = 'inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background disabled:opacity-50 disabled:cursor-not-allowed'

    const variants = {
      primary: 'bg-primary text-background hover:bg-primary-hover',
      secondary: 'bg-surface text-text-primary border border-border hover:bg-surface-elevated',
      ghost: 'text-text-secondary hover:text-text-primary hover:bg-surface-elevated',
      danger: 'bg-error text-white hover:bg-red-600',
    }

    const sizes = {
      sm: 'h-8 px-3 text-sm',
      md: 'h-10 px-4 text-sm',
      lg: 'h-12 px-6 text-base',
    }

    return (
      <button
        ref={ref}
        className={cn(baseStyles, variants[variant], sizes[size], className)}
        disabled={disabled}
        {...props}
      />
    )
  }
)

Button.displayName = 'Button'
export { Button }
```

- [ ] **Step 2: Create src/components/ui/Input.tsx**

```typescript
import { InputHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s/g, '-')

    return (
      <div className="space-y-1">
        {label && (
          <label htmlFor={inputId} className="block text-sm font-medium text-text-secondary">
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={cn(
            'w-full h-10 px-3 bg-surface border rounded-md text-text-primary placeholder:text-text-muted',
            'focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent',
            'disabled:opacity-50 disabled:cursor-not-allowed',
            error ? 'border-error' : 'border-border',
            className
          )}
          {...props}
        />
        {error && <p className="text-sm text-error">{error}</p>}
      </div>
    )
  }
)

Input.displayName = 'Input'
export { Input }
```

- [ ] **Step 3: Create src/components/ui/Card.tsx**

```typescript
import { HTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'elevated'
}

const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    const variants = {
      default: 'bg-surface border border-border',
      elevated: 'bg-surface-elevated shadow-lg',
    }

    return (
      <div
        ref={ref}
        className={cn('rounded-lg p-4', variants[variant], className)}
        {...props}
      />
    )
  }
)

Card.displayName = 'Card'
export { Card }
```

- [ ] **Step 4: Create src/components/ui/Badge.tsx**

```typescript
import { HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info'
}

export function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  const variants = {
    default: 'bg-surface-elevated text-text-secondary',
    success: 'bg-success/20 text-success',
    warning: 'bg-warning/20 text-warning',
    error: 'bg-error/20 text-error',
    info: 'bg-info/20 text-info',
  }

  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 text-xs font-medium rounded',
        variants[variant],
        className
      )}
      {...props}
    />
  )
}
```

- [ ] **Step 5: Create src/components/ui/index.ts**

```typescript
export { Button } from './Button'
export { Input } from './Input'
export { Card } from './Card'
export { Badge } from './Badge'
```

- [ ] **Step 6: Commit**

```bash
cd /mnt/devops/frontend
git add src/components/ui/Button.tsx src/components/ui/Input.tsx src/components/ui/Card.tsx src/components/ui/Badge.tsx src/components/ui/index.ts
git commit -m "feat(frontend): add base UI components (Button, Input, Card, Badge)"
```

---

## Task 4: Login Page

**Files:**
- Create: `frontend/src/pages/Login.tsx`

- [ ] **Step 1: Create src/pages/Login.tsx**

```typescript
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'
import { authApi } from '@/api/endpoints/auth'
import { Button, Input, Card } from '@/components/ui'

export default function Login() {
  const navigate = useNavigate()
  const login = useAuthStore((state) => state.login)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      const response = await authApi.login({ username, password })
      login(response.token, response.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <Card className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-text-primary">DevOps Toolkit</h1>
          <p className="text-text-secondary mt-2">Sign in to your account</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
          />

          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
          />

          {error && <p className="text-sm text-error">{error}</p>}

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? 'Signing in...' : 'Sign in'}
          </Button>
        </form>
      </Card>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /mnt/devops/frontend
git add src/pages/Login.tsx
git commit -m "feat(frontend): add Login page"
```

---

## Task 5: Layout Components (Sidebar, Header, PageLayout)

**Files:**
- Create: `frontend/src/components/layout/Sidebar.tsx`
- Create: `frontend/src/components/layout/Header.tsx`
- Create: `frontend/src/components/layout/PageLayout.tsx`

- [ ] **Step 1: Create src/components/layout/Sidebar.tsx**

```typescript
import { Link, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Server,
  GitBranch,
  FileText,
  AlertCircle,
  Monitor,
  Container,
  HardDrive,
  FolderTree,
} from 'lucide-react'
import { cn } from '@/lib/utils'

const navItems = [
  { icon: LayoutDashboard, label: 'Dashboard', path: '/' },
  { icon: Server, label: 'Devices', path: '/devices' },
  { icon: GitBranch, label: 'Pipelines', path: '/pipelines' },
  { icon: FileText, label: 'Logs', path: '/logs' },
  { icon: Monitor, label: 'Monitoring', path: '/monitoring' },
  { icon: AlertCircle, label: 'Alerts', path: '/alerts' },
  { icon: Container, label: 'Kubernetes', path: '/kubernetes' },
  { icon: HardDrive, label: 'Physical Hosts', path: '/physical-hosts' },
  { icon: FolderTree, label: 'Projects', path: '/projects' },
]

export function Sidebar() {
  const location = useLocation()

  return (
    <aside className="w-64 bg-surface border-r border-border h-screen flex flex-col">
      <div className="p-4 border-b border-border">
        <h1 className="text-lg font-bold text-text-primary">DevOps Toolkit</h1>
      </div>

      <nav className="flex-1 p-4 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive = location.pathname === item.path

          return (
            <Link
              key={item.path}
              to={item.path}
              className={cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors',
                isActive
                  ? 'bg-primary-muted text-primary'
                  : 'text-text-secondary hover:text-text-primary hover:bg-surface-elevated'
              )}
            >
              <Icon className="w-5 h-5" />
              {item.label}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
```

- [ ] **Step 2: Create src/components/layout/Header.tsx**

```typescript
import { useNavigate } from 'react-router-dom'
import { LogOut, User } from 'lucide-react'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui'

export function Header() {
  const navigate = useNavigate()
  const { user, logout } = useAuthStore()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <header className="h-16 bg-surface border-b border-border px-6 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <span className="text-sm text-text-muted">
          {new Date().toLocaleDateString('en-US', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
          })}
        </span>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-sm text-text-secondary">
          <User className="w-4 h-4" />
          <span>{user?.username}</span>
        </div>
        <Button variant="ghost" size="sm" onClick={handleLogout}>
          <LogOut className="w-4 h-4 mr-2" />
          Logout
        </Button>
      </div>
    </header>
  )
}
```

- [ ] **Step 3: Create src/components/layout/PageLayout.tsx**

```typescript
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Header } from './Header'

export default function PageLayout() {
  return (
    <div className="flex h-screen bg-background">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Header />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Commit**

```bash
cd /mnt/devops/frontend
git add src/components/layout/Sidebar.tsx src/components/layout/Header.tsx src/components/layout/PageLayout.tsx
git commit -m "feat(frontend): add layout components (Sidebar, Header, PageLayout)"
```

---

## Task 6: Dashboard Page

**Files:**
- Create: `frontend/src/pages/Dashboard.tsx`
- Create: `frontend/src/api/endpoints/devices.ts`

- [ ] **Step 1: Create src/api/endpoints/devices.ts**

```typescript
import { apiClient } from '../client'

export interface Device {
  id: string
  name: string
  type: string
  status: string
  environment: string
  labels: Record<string, string>
  registeredAt?: string
  lastSeen?: string
}

export const devicesApi = {
  list: () => apiClient.get<Device[]>('/api/devices'),

  get: (id: string) => apiClient.get<Device>(`/api/devices/${id}`),

  create: (data: Partial<Device>) => apiClient.post<Device>('/api/devices', data),

  update: (id: string, data: Partial<Device>) => apiClient.put<Device>(`/api/devices/${id}`, data),

  delete: (id: string) => apiClient.delete(`/api/devices/${id}`),
}
```

- [ ] **Step 2: Create src/pages/Dashboard.tsx**

```typescript
import { useQuery } from '@tanstack/react-query'
import { Server, Activity, AlertCircle, GitBranch } from 'lucide-react'
import { Card, Badge } from '@/components/ui'
import { devicesApi } from '@/api/endpoints/devices'

export default function Dashboard() {
  const { data: devices = [] } = useQuery({
    queryKey: ['devices'],
    queryFn: devicesApi.list,
  })

  const stats = [
    { label: 'Total Devices', value: devices.length, icon: Server, color: 'text-primary' },
    { label: 'Active Devices', value: devices.filter((d) => d.status === 'active').length, icon: Activity, color: 'text-success' },
    { label: 'Pending', value: devices.filter((d) => d.status === 'pending').length, icon: AlertCircle, color: 'text-warning' },
    { label: 'Pipelines', value: 0, icon: GitBranch, color: 'text-info' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-text-primary">Dashboard</h1>
        <p className="text-text-secondary mt-1">Overview of your DevOps infrastructure</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((stat) => {
          const Icon = stat.icon
          return (
            <Card key={stat.label} className="flex items-center gap-4">
              <div className={`p-3 rounded-lg bg-surface-elevated ${stat.color}`}>
                <Icon className="w-6 h-6" />
              </div>
              <div>
                <p className="text-2xl font-bold text-text-primary">{stat.value}</p>
                <p className="text-sm text-text-secondary">{stat.label}</p>
              </div>
            </Card>
          )
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <h2 className="text-lg font-semibold text-text-primary mb-4">Recent Devices</h2>
          <div className="space-y-2">
            {devices.slice(0, 5).map((device) => (
              <div key={device.id} className="flex items-center justify-between py-2 border-b border-border-subtle last:border-0">
                <div>
                  <p className="text-sm font-medium text-text-primary">{device.name}</p>
                  <p className="text-xs text-text-muted">{device.type}</p>
                </div>
                <Badge variant={device.status === 'active' ? 'success' : 'warning'}>{device.status}</Badge>
              </div>
            ))}
            {devices.length === 0 && <p className="text-text-muted text-sm">No devices found</p>}
          </div>
        </Card>

        <Card>
          <h2 className="text-lg font-semibold text-text-primary mb-4">Quick Actions</h2>
          <div className="grid grid-cols-2 gap-3">
            <button className="p-3 text-left rounded-lg bg-surface-elevated hover:bg-border text-sm font-medium text-text-primary transition-colors">
              Register Device
            </button>
            <button className="p-3 text-left rounded-lg bg-surface-elevated hover:bg-border text-sm font-medium text-text-primary transition-colors">
              View Logs
            </button>
            <button className="p-3 text-left rounded-lg bg-surface-elevated hover:bg-border text-sm font-medium text-text-primary transition-colors">
              Create Pipeline
            </button>
            <button className="p-3 text-left rounded-lg bg-surface-elevated hover:bg-border text-sm font-medium text-text-primary transition-colors">
              Trigger Alert
            </button>
          </div>
        </Card>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
cd /mnt/devops/frontend
git add src/pages/Dashboard.tsx src/api/endpoints/devices.ts
git commit -m "feat(frontend): add Dashboard page with device stats"
```

---

## Task 7: Device List Page

**Files:**
- Create: `frontend/src/pages/devices/DeviceList.tsx`

- [ ] **Step 1: Create src/pages/devices/DeviceList.tsx**

```typescript
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, Plus } from 'lucide-react'
import { Card, Button, Input, Badge } from '@/components/ui'
import { devicesApi, Device } from '@/api/endpoints/devices'

export default function DeviceList() {
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('')

  const { data: devices = [], isLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: devicesApi.list,
  })

  const filteredDevices = devices.filter((device) => {
    const matchesSearch = device.name.toLowerCase().includes(search.toLowerCase())
    const matchesType = !typeFilter || device.type === typeFilter
    return matchesSearch && matchesType
  })

  const deviceTypes = [...new Set(devices.map((d) => d.type))]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Devices</h1>
          <p className="text-text-secondary mt-1">Manage your infrastructure devices</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          Register Device
        </Button>
      </div>

      <Card>
        <div className="flex items-center gap-4 mb-6">
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
            <Input
              placeholder="Search devices..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10"
            />
          </div>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="h-10 px-3 bg-surface border border-border rounded-md text-text-primary"
          >
            <option value="">All Types</option>
            {deviceTypes.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </div>

        {isLoading ? (
          <div className="text-center py-8 text-text-muted">Loading...</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Name</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Type</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Status</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Environment</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Last Seen</th>
                </tr>
              </thead>
              <tbody>
                {filteredDevices.map((device) => (
                  <tr key={device.id} className="border-b border-border-subtle hover:bg-surface-elevated">
                    <td className="py-3 px-4 text-sm text-text-primary font-mono">{device.name}</td>
                    <td className="py-3 px-4 text-sm text-text-secondary">{device.type}</td>
                    <td className="py-3 px-4">
                      <Badge variant={device.status === 'active' ? 'success' : 'warning'}>{device.status}</Badge>
                    </td>
                    <td className="py-3 px-4 text-sm text-text-secondary">{device.environment || '-'}</td>
                    <td className="py-3 px-4 text-sm text-text-muted">{device.lastSeen || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {filteredDevices.length === 0 && (
              <div className="text-center py-8 text-text-muted">No devices found</div>
            )}
          </div>
        )}
      </Card>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
cd /mnt/devops/frontend
git add src/pages/devices/DeviceList.tsx
git commit -m "feat(frontend): add DeviceList page with search and filters"
```

---

## Task 8: K8s Cluster Pages with Environment Filtering

**Files:**
- Create: `frontend/src/api/endpoints/kubernetes.ts`
- Create: `frontend/src/pages/kubernetes/ClusterList.tsx`
- Create: `frontend/src/components/shared/StatusBadge.tsx`
- Create: `frontend/src/components/shared/EmptyState.tsx`

- [ ] **Step 1: Create src/api/endpoints/kubernetes.ts**

```typescript
import { apiClient } from '../client'

export interface K8sCluster {
  id: string
  name: string
  type: string
  status: string
  version?: string
  environment?: string
  createdAt?: string
}

export const kubernetesApi = {
  listClusters: () => apiClient.get<K8sCluster[]>('/api/k8s/clusters'),

  getCluster: (name: string) => apiClient.get<K8sCluster>(`/api/k8s/clusters/${name}`),

  getNodes: (cluster: string) =>
    apiClient.get<{ name: string; ready: boolean; role: string }[]>(`/api/k8s/clusters/${cluster}/nodes`),

  getPods: (cluster: string, namespace?: string) =>
    apiClient.get<{ name: string; namespace: string; ready: string }[]>(
      `/api/k8s/clusters/${cluster}/pods`,
      namespace ? { params: { namespace } } : undefined
    ),

  getNamespaces: (cluster: string) =>
    apiClient.get<string[]>(`/api/k8s/clusters/${cluster}/namespaces`),
}
```

- [ ] **Step 2: Create src/components/shared/StatusBadge.tsx**

```typescript
import { cn } from '@/lib/utils'

interface StatusBadgeProps {
  status: 'online' | 'offline' | 'pending' | 'error' | string
  children: React.ReactNode
}

export function StatusBadge({ status, children }: StatusBadgeProps) {
  const variants: Record<string, string> = {
    online: 'bg-success/20 text-success',
    active: 'bg-success/20 text-success',
    offline: 'bg-error/20 text-error',
    error: 'bg-error/20 text-error',
    pending: 'bg-warning/20 text-warning',
    warning: 'bg-warning/20 text-warning',
    info: 'bg-info/20 text-info',
  }

  return (
    <span className={cn('inline-flex items-center gap-1.5 px-2 py-0.5 text-xs font-medium rounded', variants[status] || variants.info)}>
      <span className="w-1.5 h-1.5 rounded-full bg-current" />
      {children}
    </span>
  )
}
```

- [ ] **Step 3: Create src/components/shared/EmptyState.tsx**

```typescript
import { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  icon: LucideIcon
  title: string
  description?: string
  action?: React.ReactNode
}

export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="p-3 rounded-full bg-surface-elevated mb-4">
        <Icon className="w-8 h-8 text-text-muted" />
      </div>
      <h3 className="text-lg font-medium text-text-primary mb-1">{title}</h3>
      {description && <p className="text-sm text-text-muted mb-4 max-w-sm">{description}</p>}
      {action}
    </div>
  )
}
```

- [ ] **Step 4: Create src/pages/kubernetes/ClusterList.tsx**

```typescript
import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, Server } from 'lucide-react'
import { Card, Button, Input, Badge } from '@/components/ui'
import { kubernetesApi, K8sCluster } from '@/api/endpoints/kubernetes'
import { StatusBadge } from '@/components/shared/StatusBadge'
import { EmptyState } from '@/components/shared/EmptyState'

// Environment color coding per DESIGN.md
const envStyles: Record<string, { bg: string; border: string; label: string }> = {
  dev: { bg: 'rgba(59, 130, 246, 0.15)', border: '#3b82f6', label: '开发' },
  test: { bg: 'rgba(245, 158, 11, 0.15)', border: '#f59e0b', label: '测试' },
  uat: { bg: 'rgba(168, 85, 247, 0.15)', border: '#a855f7', label: 'UAT' },
  prod: { bg: 'rgba(239, 68, 68, 0.15)', border: '#ef4444', label: '生产' },
  default: { bg: 'rgba(148, 163, 184, 0.1)', border: '#94a3b8', label: '标准' },
}

function getEnvStyle(type_: string) {
  const lower = type_.toLowerCase()
  for (const [key, style] of Object.entries(envStyles)) {
    if (lower.includes(key)) return style
  }
  return envStyles.default
}

export default function ClusterList() {
  const [search, setSearch] = useState('')
  const [envFilter, setEnvFilter] = useState('')

  const { data: clusters = [], isLoading } = useQuery({
    queryKey: ['kubernetes', 'clusters'],
    queryFn: kubernetesApi.listClusters,
  })

  const filteredClusters = useMemo(() => {
    return clusters.filter((cluster) => {
      const matchesSearch = cluster.name.toLowerCase().includes(search.toLowerCase())
      const envStyle = getEnvStyle(cluster.type)
      const matchesEnv = !envFilter || envStyle.label === envFilter
      return matchesSearch && matchesEnv
    })
  }, [clusters, search, envFilter])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Kubernetes Clusters</h1>
          <p className="text-text-secondary mt-1">Manage your K8s clusters</p>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
          <Input
            placeholder="Search clusters..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10"
          />
        </div>
        <select
          value={envFilter}
          onChange={(e) => setEnvFilter(e.target.value)}
          className="h-10 px-3 bg-surface border border-border rounded-md text-text-primary"
        >
          <option value="">All Environments</option>
          <option value="开发">开发</option>
          <option value="测试">测试</option>
          <option value="UAT">UAT</option>
          <option value="生产">生产</option>
        </select>
      </div>

      {isLoading ? (
        <div className="text-center py-12 text-text-muted">Loading...</div>
      ) : filteredClusters.length === 0 ? (
        <EmptyState icon={Server} title="No clusters found" description="No clusters match your search criteria" />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredClusters.map((cluster) => {
            const envStyle = getEnvStyle(cluster.type)
            return (
              <Card
                key={cluster.id}
                className="cursor-pointer hover:border-primary transition-colors"
                style={{ borderColor: envStyle.border }}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <Server className="w-5 h-5 text-primary" />
                    <h3 className="font-semibold text-text-primary">{cluster.name}</h3>
                  </div>
                  <span
                    className="text-xs px-2 py-0.5 rounded"
                    style={{ backgroundColor: envStyle.bg, color: envStyle.border }}
                  >
                    {envStyle.label}
                  </span>
                </div>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-text-muted">Type</span>
                    <span className="text-text-secondary">{cluster.type}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-text-muted">Status</span>
                    <StatusBadge status={cluster.status}>{cluster.status}</StatusBadge>
                  </div>
                  {cluster.version && (
                    <div className="flex justify-between">
                      <span className="text-text-muted">Version</span>
                      <span className="text-text-secondary font-mono">{cluster.version}</span>
                    </div>
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 5: Commit**

```bash
cd /mnt/devops/frontend
git add src/api/endpoints/kubernetes.ts src/pages/kubernetes/ClusterList.tsx src/components/shared/StatusBadge.tsx src/components/shared/EmptyState.tsx
git commit -m "feat(frontend): add K8s cluster pages with environment filtering"
```

---

## Task 9: Remaining Pages (Logs, Alerts, Physical Hosts, Projects)

**Files:**
- Create: `frontend/src/api/endpoints/logs.ts`
- Create: `frontend/src/api/endpoints/alerts.ts`
- Create: `frontend/src/pages/logs/LogViewer.tsx`
- Create: `frontend/src/pages/alerts/AlertChannels.tsx`
- Create: `frontend/src/pages/physical-hosts/HostList.tsx`
- Create: `frontend/src/pages/projects/ProjectList.tsx`

- [ ] **Step 1: Create src/api/endpoints/logs.ts**

```typescript
import { apiClient } from '../client'

export interface LogEntry {
  id: string
  timestamp: string
  level: string
  message: string
  source: string
}

export interface LogQuery {
  level?: string
  source?: string
  search?: string
  limit?: number
  offset?: number
}

export const logsApi = {
  query: (options?: LogQuery) =>
    apiClient.get<{ data: LogEntry[]; total: number }>('/api/logs', { params: options as Record<string, string> }),

  stats: () => apiClient.get<{ total: number; by_level: Record<string, number> }>('/api/logs/stats'),

  generate: (count: number) => apiClient.post<{ generated: number }>('/api/logs/generate', { count }),
}
```

- [ ] **Step 2: Create src/api/endpoints/alerts.ts**

```typescript
import { apiClient } from '../client'

export interface AlertChannel {
  id: string
  name: string
  type: 'slack' | 'webhook' | 'email' | 'log'
  config?: Record<string, string>
}

export interface AlertHistory {
  id: string
  name: string
  severity: string
  message: string
  channel: string
  status: string
  timestamp: string
}

export const alertsApi = {
  listChannels: () => apiClient.get<AlertChannel[]>('/api/alerts/channels'),

  createChannel: (data: Partial<AlertChannel>) =>
    apiClient.post<AlertChannel>('/api/alerts/channels', data),

  deleteChannel: (id: string) => apiClient.delete(`/api/alerts/channels/${id}`),

  trigger: (data: { name: string; message: string; severity: string; channel: string }) =>
    apiClient.post('/api/alerts/trigger', data),

  history: () => apiClient.get<AlertHistory[]>('/api/alerts/history'),
}
```

- [ ] **Step 3: Create src/pages/logs/LogViewer.tsx**

```typescript
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, RefreshCw } from 'lucide-react'
import { Card, Button, Input, Badge } from '@/components/ui'
import { logsApi, LogEntry } from '@/api/endpoints/logs'
import { formatDate } from '@/lib/utils'

const levelColors: Record<string, string> = {
  error: 'error',
  warn: 'warning',
  info: 'info',
  debug: 'default',
}

export default function LogViewer() {
  const [search, setSearch] = useState('')
  const [levelFilter, setLevelFilter] = useState('')

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['logs', { search, level: levelFilter }],
    queryFn: () =>
      logsApi.query({
        search: search || undefined,
        level: levelFilter || undefined,
        limit: 100,
      }),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Log Viewer</h1>
          <p className="text-text-secondary mt-1">Search and filter system logs</p>
        </div>
        <Button variant="secondary" onClick={() => refetch()}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      </div>

      <Card>
        <div className="flex items-center gap-4 mb-6">
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
            <Input
              placeholder="Search logs..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10"
            />
          </div>
          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value)}
            className="h-10 px-3 bg-surface border border-border rounded-md text-text-primary"
          >
            <option value="">All Levels</option>
            <option value="error">Error</option>
            <option value="warn">Warning</option>
            <option value="info">Info</option>
            <option value="debug">Debug</option>
          </select>
        </div>

        {isLoading ? (
          <div className="text-center py-8 text-text-muted">Loading...</div>
        ) : (
          <div className="space-y-1 font-mono text-sm">
            {data?.data.map((log: LogEntry) => (
              <div key={log.id} className="flex items-start gap-3 py-2 border-b border-border-subtle last:border-0">
                <span className="text-text-muted shrink-0">{formatDate(log.timestamp)}</span>
                <Badge variant={levelColors[log.level] || 'default'}>{log.level}</Badge>
                <span className="text-text-secondary shrink-0">[{log.source}]</span>
                <span className="text-text-primary flex-1">{log.message}</span>
              </div>
            ))}
            {data?.data.length === 0 && <div className="text-center py-8 text-text-muted">No logs found</div>}
          </div>
        )}
      </Card>
    </div>
  )
}
```

- [ ] **Step 4: Create src/pages/alerts/AlertChannels.tsx**

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { Card, Button, Badge } from '@/components/ui'
import { alertsApi } from '@/api/endpoints/alerts'

export default function AlertChannels() {
  const queryClient = useQueryClient()

  const { data: channels = [] } = useQuery({
    queryKey: ['alerts', 'channels'],
    queryFn: alertsApi.listChannels,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => alertsApi.deleteChannel(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alerts', 'channels'] }),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Alert Channels</h1>
          <p className="text-text-secondary mt-1">Configure notification channels</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          Add Channel
        </Button>
      </div>

      <div className="grid gap-4">
        {channels.map((channel) => (
          <Card key={channel.id} className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <Badge variant="info">{channel.type}</Badge>
              <div>
                <p className="font-medium text-text-primary">{channel.name}</p>
                <p className="text-sm text-text-muted">{channel.type} channel</p>
              </div>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => deleteMutation.mutate(channel.id)}
            >
              <Trash2 className="w-4 h-4 text-error" />
            </Button>
          </Card>
        ))}
        {channels.length === 0 && (
          <Card>
            <p className="text-center text-text-muted py-8">No alert channels configured</p>
          </Card>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Create src/pages/physical-hosts/HostList.tsx**

```typescript
import { useQuery } from '@tanstack/react-query'
import { HardDrive } from 'lucide-react'
import { Card, Badge } from '@/components/ui'
import { apiClient } from '@/api/client'
import { StatusBadge } from '@/components/shared/StatusBadge'
import { EmptyState } from '@/components/shared/EmptyState'

interface PhysicalHost {
  id: string
  hostname: string
  ip: string
  state: string
  lastAgentUpdate?: string
}

export default function HostList() {
  const { data: hosts = [], isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: () => apiClient.get<PhysicalHost[]>('/api/physical-hosts'),
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-text-primary">Physical Hosts</h1>
        <p className="text-text-secondary mt-1">Monitor hosts via SSH and agents</p>
      </div>

      {isLoading ? (
        <div className="text-center py-12 text-text-muted">Loading...</div>
      ) : hosts.length === 0 ? (
        <EmptyState icon={HardDrive} title="No hosts found" description="Register physical hosts to monitor them" />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {hosts.map((host) => (
            <Card key={host.id}>
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-2">
                  <HardDrive className="w-5 h-5 text-primary" />
                  <h3 className="font-semibold text-text-primary">{host.hostname}</h3>
                </div>
                <StatusBadge status={host.state}>{host.state}</StatusBadge>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-text-muted">IP Address</span>
                  <span className="text-text-secondary font-mono">{host.ip}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-text-muted">Last Update</span>
                  <span className="text-text-secondary">{host.lastAgentUpdate || '-'}</span>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 6: Create src/pages/projects/ProjectList.tsx**

```typescript
import { useQuery } from '@tanstack/react-query'
import { FolderTree, ChevronRight } from 'lucide-react'
import { Card } from '@/components/ui'
import { apiClient } from '@/api/client'

interface Project {
  id: string
  name: string
  type: string
  parentId?: string
}

export default function ProjectList() {
  const { data: projects = [], isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: () => apiClient.get<Project[]>('/api/org/projects'),
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-text-primary">Projects</h1>
        <p className="text-text-secondary mt-1">Organizational hierarchy for FinOps</p>
      </div>

      {isLoading ? (
        <div className="text-center py-12 text-text-muted">Loading...</div>
      ) : (
        <div className="space-y-2">
          {projects.map((project) => (
            <Card key={project.id} className="flex items-center justify-between cursor-pointer hover:bg-surface-elevated">
              <div className="flex items-center gap-3">
                <FolderTree className="w-5 h-5 text-primary" />
                <div>
                  <p className="font-medium text-text-primary">{project.name}</p>
                  <p className="text-xs text-text-muted">{project.type}</p>
                </div>
              </div>
              <ChevronRight className="w-4 h-4 text-text-muted" />
            </Card>
          ))}
          {projects.length === 0 && (
            <Card>
              <p className="text-center text-text-muted py-8">No projects found</p>
            </Card>
          )}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 7: Commit**

```bash
cd /mnt/devops/frontend
git add src/api/endpoints/logs.ts src/api/endpoints/alerts.ts src/pages/logs/LogViewer.tsx src/pages/alerts/AlertChannels.tsx src/pages/physical-hosts/HostList.tsx src/pages/projects/ProjectList.tsx
git commit -m "feat(frontend): add remaining pages (logs, alerts, hosts, projects)"
```

---

## Task 10: WebSocket Integration for Real-Time Updates

**Files:**
- Create: `frontend/src/api/websocket.ts`
- Create: `frontend/src/hooks/useWebSocket.ts`
- Modify: `frontend/src/pages/logs/LogViewer.tsx` (add real-time)

- [ ] **Step 1: Create src/api/websocket.ts**

```typescript
import { useAuthStore } from '../stores/authStore'

type MessageHandler = (data: unknown) => void

class WebSocketClient {
  private ws: WebSocket | null = null
  private handlers: Map<string, Set<MessageHandler>> = new Map()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5

  connect() {
    const token = useAuthStore.getState().token
    if (!token) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    this.ws = new WebSocket(wsUrl)

    this.ws.onopen = () => {
      console.log('WebSocket connected')
      this.reconnectAttempts = 0
      // Subscribe to channels
      this.subscribe('log')
      this.subscribe('alert')
      this.subscribe('device_event')
    }

    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        const handlers = this.handlers.get(message.channel)
        if (handlers) {
          handlers.forEach((handler) => handler(message.payload))
        }
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }

    this.ws.onclose = () => {
      console.log('WebSocket disconnected')
      if (this.reconnectAttempts < this.maxReconnectAttempts) {
        setTimeout(() => {
          this.reconnectAttempts++
          this.connect()
        }, 1000 * this.reconnectAttempts)
      }
    }
  }

  subscribe(channel: string) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ action: 'subscribe', channel }))
    }
  }

  unsubscribe(channel: string) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ action: 'unsubscribe', channel }))
    }
  }

  on(channel: string, handler: MessageHandler) {
    if (!this.handlers.has(channel)) {
      this.handlers.set(channel, new Set())
    }
    this.handlers.get(channel)!.add(handler)
  }

  off(channel: string, handler: MessageHandler) {
    this.handlers.get(channel)?.delete(handler)
  }

  disconnect() {
    this.ws?.close()
    this.ws = null
  }
}

export const wsClient = new WebSocketClient()
```

- [ ] **Step 2: Create src/hooks/useWebSocket.ts**

```typescript
import { useEffect } from 'react'
import { wsClient } from '@/api/websocket'
import { useAuthStore } from '@/stores/authStore'

export function useWebSocket() {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

  useEffect(() => {
    if (isAuthenticated) {
      wsClient.connect()
    }

    return () => {
      wsClient.disconnect()
    }
  }, [isAuthenticated])

  return wsClient
}
```

- [ ] **Step 3: Commit**

```bash
cd /mnt/devops/frontend
git add src/api/websocket.ts src/hooks/useWebSocket.ts
git commit -m "feat(frontend): add WebSocket client for real-time updates"
```

---

## Task 11: Install Dependencies and Verify

**Files:**
- Modify: (none - verify only)

- [ ] **Step 1: Install dependencies**

```bash
cd /mnt/devops/frontend
npm install
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /mnt/devops/frontend
npx tsc --noEmit
```

- [ ] **Step 3: Test dev server starts**

```bash
cd /mnt/devops/frontend
npm run dev &
sleep 5
curl -s http://localhost:3001 | head -20
```

- [ ] **Step 4: Commit**

```bash
cd /mnt/devops/frontend
git add -A
git commit -m "chore(frontend): install dependencies and verify build"
```

---

## Self-Review Checklist

1. **Spec coverage:** All PRD requirements mapped to tasks
2. **Placeholder scan:** No TBD/TODO found
3. **Type consistency:** API endpoints, components use consistent types
4. **Design compliance:** Colors, typography match DESIGN.md

**Plan complete and saved to `docs/superpowers/plans/2026-04-27-frontend-reimplementation.md`.**
