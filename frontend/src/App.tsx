import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { useAuthStore } from './stores/authStore'
import { AppShell } from './components/layout/AppShell'
import { Login } from './pages/Login'
import { ToastProvider } from './components/ui/Toast'

// Lazy load all page components
const Dashboard = lazy(() => import('./pages/Dashboard').then(m => ({ default: m.Dashboard })))
const DeviceList = lazy(() => import('./pages/devices/DeviceList').then(m => ({ default: m.DeviceList })))
const DevicesDetail = lazy(() => import('./pages/devices/DeviceDetail').then(m => ({ default: m.DeviceDetail })))
const HostList = lazy(() => import('./pages/physical-hosts/HostList').then(m => ({ default: m.HostList })))
const HostDetail = lazy(() => import('./pages/physical-hosts/HostDetail').then(m => ({ default: m.HostDetail })))
const HostServices = lazy(() => import('./pages/physical-hosts/HostServices').then(m => ({ default: m.HostServices })))
const HostConfig = lazy(() => import('./pages/physical-hosts/HostConfig').then(m => ({ default: m.HostConfig })))
const PipelineList = lazy(() => import('./pages/pipelines/PipelineList').then(m => ({ default: m.PipelineList })))
const PipelineDetail = lazy(() => import('./pages/pipelines/PipelineDetail').then(m => ({ default: m.PipelineDetail })))
const PipelineRun = lazy(() => import('./pages/pipelines/PipelineRun').then(m => ({ default: m.PipelineRun })))
const LogViewer = lazy(() => import('./pages/logs/LogViewer').then(m => ({ default: m.LogViewer })))
const LogAlerts = lazy(() => import('./pages/logs/LogAlerts').then(m => ({ default: m.default })))
const AlertChannels = lazy(() => import('./pages/alerts/AlertChannels').then(m => ({ default: m.AlertChannels })))
const AlertHistory = lazy(() => import('./pages/alerts/AlertHistory').then(m => ({ default: m.AlertHistory })))
const ClusterList = lazy(() => import('./pages/kubernetes/ClusterList').then(m => ({ default: m.ClusterList })))
const ClusterDetail = lazy(() => import('./pages/kubernetes/ClusterDetail').then(m => ({ default: m.ClusterDetail })))
const ClusterNodes = lazy(() => import('./pages/kubernetes/ClusterNodes').then(m => ({ default: m.ClusterNodes })))
const ClusterNamespaces = lazy(() => import('./pages/kubernetes/ClusterNamespaces').then(m => ({ default: m.ClusterNamespaces })))
const ClusterPods = lazy(() => import('./pages/kubernetes/ClusterPods').then(m => ({ default: m.ClusterPods })))
const PodLogs = lazy(() => import('./pages/kubernetes/PodLogs').then(m => ({ default: m.PodLogs })))
const PodExec = lazy(() => import('./pages/kubernetes/PodExec').then(m => ({ default: m.PodExec })))
const ProjectList = lazy(() => import('./pages/projects/ProjectList').then(m => ({ default: m.ProjectList })))
const ProjectDetail = lazy(() => import('./pages/projects/ProjectDetail').then(m => ({ default: m.ProjectDetail })))
const ProjectResources = lazy(() => import('./pages/projects/ProjectResources').then(m => ({ default: m.ProjectResources })))
const ProjectPermissions = lazy(() => import('./pages/projects/ProjectPermissions').then(m => ({ default: m.ProjectPermissions })))
const FinOps = lazy(() => import('./pages/reports/FinOps').then(m => ({ default: m.FinOps })))
const AuditLogs = lazy(() => import('./pages/reports/AuditLogs').then(m => ({ default: m.AuditLogs })))
const Settings = lazy(() => import('./pages/settings/Settings').then(m => ({ default: m.Settings })))

function LoadingSpinner() {
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100vh',
      color: 'var(--color-text-secondary)'
    }}>
      Loading...
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  const _hasHydrated = useAuthStore((state) => state._hasHydrated)

  // Show nothing while hydrating to prevent flash
  if (!_hasHydrated) {
    return <LoadingSpinner />
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export default function App() {
  return (
    <ToastProvider>
      <Suspense fallback={<LoadingSpinner />}>
        <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <AppShell />
            </ProtectedRoute>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="devices" element={<DeviceList />} />
          <Route path="devices/:id" element={<DevicesDetail />} />
          <Route path="physical-hosts" element={<HostList />} />
          <Route path="physical-hosts/:id" element={<HostDetail />} />
          <Route path="physical-hosts/:id/services" element={<HostServices />} />
          <Route path="physical-hosts/:id/config" element={<HostConfig />} />
          <Route path="pipelines" element={<PipelineList />} />
          <Route path="pipelines/:id" element={<PipelineDetail />} />
          <Route path="pipelines/:id/run" element={<PipelineRun />} />
          <Route path="logs" element={<LogViewer />} />
          <Route path="logs/alerts" element={<LogAlerts />} />
          <Route path="alerts" element={<AlertChannels />} />
          <Route path="alerts/history" element={<AlertHistory />} />
          <Route path="k8s" element={<ClusterList />} />
          <Route path="k8s/:cluster" element={<ClusterDetail />} />
          <Route path="k8s/:cluster/nodes" element={<ClusterNodes />} />
          <Route path="k8s/:cluster/namespaces" element={<ClusterNamespaces />} />
          <Route path="k8s/:cluster/pods" element={<ClusterPods />} />
          <Route path="k8s/:cluster/pods/:pod/logs" element={<PodLogs />} />
          <Route path="k8s/:cluster/pods/:pod/exec" element={<PodExec />} />
          <Route path="projects" element={<ProjectList />} />
          <Route path="projects/:id" element={<ProjectDetail />} />
          <Route path="projects/:id/resources" element={<ProjectResources />} />
          <Route path="projects/:id/permissions" element={<ProjectPermissions />} />
          <Route path="reports/finops" element={<FinOps />} />
          <Route path="audit-logs" element={<AuditLogs />} />
          <Route path="settings" element={<Settings />} />
        </Route>
      </Routes>
    </Suspense>
    </ToastProvider>
  )
}