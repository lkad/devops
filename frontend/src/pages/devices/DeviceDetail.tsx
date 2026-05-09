import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, RefreshCw, Server, Box, CheckCircle, XCircle,
  Pause, Play, Wrench, AlertCircle, AlertTriangle, Info, Bug
} from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi, type Device, type K8sNode, type K8sPod } from '@/api/endpoints/devices'
import { logsApi } from '@/api/endpoints/logs'

// Status display helpers
const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
    case 'healthy':
    case 'running':
      return 'success'
    case 'pending':
    case 'unknown':
      return 'warning'
    case 'authenticated':
    case 'registered':
      return 'info'
    case 'inactive':
    case 'offline':
    case 'failed':
    case 'unhealthy':
    case 'stopped':
      return 'error'
    case 'maintenance':
      return 'warning'
    default:
      return 'default'
  }
}

const statusIcon = (status: string) => {
  switch (status.toLowerCase()) {
    case 'active':
    case 'healthy':
    case 'running':
      return <CheckCircle className="w-4 h-4 text-[var(--color-success)]" />
    case 'pending':
    case 'unknown':
      return <Pause className="w-4 h-4 text-[var(--color-warning)]" />
    case 'authenticated':
    case 'registered':
      return <Play className="w-4 h-4 text-[var(--color-info)]" />
    case 'inactive':
    case 'offline':
    case 'failed':
    case 'unhealthy':
      return <XCircle className="w-4 h-4 text-[var(--color-error)]" />
    case 'maintenance':
      return <Wrench className="w-4 h-4 text-[var(--color-warning)]" />
    default:
      return null
  }
}

const formatTime = (timestamp?: string) => {
  if (!timestamp) return 'Never'
  return new Date(timestamp).toLocaleString()
}

// Tab configuration
interface TabConfig {
  key: string
  label: string
}

const getTabsForDeviceType = (type: string): TabConfig[] => {
  switch (type) {
    case 'k8s_cluster':
      return [
        { key: 'overview', label: 'Overview' },
        { key: 'nodes', label: 'Nodes' },
        { key: 'pods', label: 'Pods' },
        { key: 'namespaces', label: 'Namespaces' },
        { key: 'logs', label: 'Logs' },
      ]
    case 'physical_host':
      return [
        { key: 'overview', label: 'Overview' },
        { key: 'state', label: 'State Control' },
        { key: 'logs', label: 'Logs' },
      ]
    default:
      return [
        { key: 'overview', label: 'Overview' },
        { key: 'logs', label: 'Logs' },
      ]
  }
}

// Overview Tab for K8s clusters
function K8sOverviewTab({ device }: { device: Device }) {
  const { data: nodes } = useQuery({
    queryKey: ['device', device.id, 'nodes'],
    queryFn: () => devicesApi.getDeviceNodes(device.id),
    enabled: !!device.id,
  })

  const { data: namespaces } = useQuery({
    queryKey: ['device', device.id, 'namespaces'],
    queryFn: () => devicesApi.getDeviceNamespaces(device.id),
    enabled: !!device.id,
  })

  const nodeCount = (nodes as K8sNode[])?.length || 0
  const readyNodes = (nodes as K8sNode[])?.filter(n => n.ready).length || 0
  const namespaceCount = (namespaces as string[])?.length || 0

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      <Card>
        <div className="text-2xl font-bold text-[var(--color-primary)]">{nodeCount}</div>
        <div className="text-sm text-text-secondary">Total Nodes</div>
        <div className="text-xs text-text-muted mt-1">
          {readyNodes} ready
        </div>
      </Card>
      <Card>
        <div className="text-2xl font-bold text-[var(--color-primary)]">{namespaceCount}</div>
        <div className="text-sm text-text-secondary">Namespaces</div>
      </Card>
      <Card>
        <div className="text-sm font-medium text-text">{device.labels?.cluster_type || 'N/A'}</div>
        <div className="text-sm text-text-secondary">Cluster Type</div>
      </Card>
    </div>
  )
}

// Nodes Tab for K8s clusters
function K8sNodesTab({ device }: { device: Device }) {
  const { data: nodes, isLoading } = useQuery({
    queryKey: ['device', device.id, 'nodes'],
    queryFn: () => devicesApi.getDeviceNodes(device.id),
    enabled: !!device.id,
  })

  const nodeList = (nodes as K8sNode[]) || []

  if (isLoading) {
    return <div className="text-text-muted">Loading nodes...</div>
  }

  if (nodeList.length === 0) {
    return <div className="text-text-muted">No nodes found</div>
  }

  return (
    <div className="space-y-2">
      {nodeList.map((node) => (
        <Card key={node.name}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className={`p-2 rounded-lg ${node.ready ? 'bg-[var(--color-success-muted)]' : 'bg-[var(--color-error-muted)]'}`}>
                <Server className={`w-5 h-5 ${node.ready ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'}`} />
              </div>
              <div>
                <div className="font-medium text-text">{node.name}</div>
                <div className="text-xs text-text-muted">
                  Role: {node.role} | CPU: {node.cpu} | Memory: {node.memory}
                </div>
              </div>
            </div>
            <Badge variant={node.ready ? 'success' : 'error'}>
              {node.ready ? 'Ready' : 'Not Ready'}
            </Badge>
          </div>
        </Card>
      ))}
    </div>
  )
}

// Pods Tab for K8s clusters
function K8sPodsTab({ device }: { device: Device }) {
  const [namespace, setNamespace] = useState('')
  const { data: namespaces } = useQuery({
    queryKey: ['device', device.id, 'namespaces'],
    queryFn: () => devicesApi.getDeviceNamespaces(device.id),
    enabled: !!device.id,
  })

  const { data: pods, isLoading } = useQuery({
    queryKey: ['device', device.id, 'pods', namespace],
    queryFn: () => devicesApi.getDevicePods(device.id, namespace || undefined),
    enabled: !!device.id,
  })

  const podList = (pods as K8sPod[]) || []
  const namespaceList = (namespaces as string[]) || []

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <select
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
          className="px-3 py-2 bg-surface border border-border rounded-lg text-sm"
        >
          <option value="">All Namespaces</option>
          {namespaceList.map(ns => (
            <option key={ns} value={ns}>{ns}</option>
          ))}
        </select>
      </div>

      {isLoading ? (
        <div className="text-text-muted">Loading pods...</div>
      ) : podList.length === 0 ? (
        <div className="text-text-muted">No pods found</div>
      ) : (
        <div className="space-y-2">
          {podList.map((pod) => (
            <Card key={`${pod.namespace}-${pod.name}`}>
              <div className="flex items-center justify-between">
                <div>
                  <div className="font-medium text-text">{pod.name}</div>
                  <div className="text-xs text-text-muted">
                    {pod.namespace} | Node: {pod.node_name}
                  </div>
                </div>
                <div className="text-right">
                  <Badge variant={pod.status === 'Running' ? 'success' : 'warning'}>
                    {pod.ready}
                  </Badge>
                  <div className="text-xs text-text-muted mt-1">
                    Restarts: {pod.restarts}
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}

// Namespaces Tab for K8s clusters
function K8sNamespacesTab({ device }: { device: Device }) {
  const { data: namespaces, isLoading } = useQuery({
    queryKey: ['device', device.id, 'namespaces'],
    queryFn: () => devicesApi.getDeviceNamespaces(device.id),
    enabled: !!device.id,
  })

  const namespaceList = (namespaces as string[]) || []

  if (isLoading) {
    return <div className="text-text-muted">Loading namespaces...</div>
  }

  if (namespaceList.length === 0) {
    return <div className="text-text-muted">No namespaces found</div>
  }

  return (
    <div className="space-y-2">
      {namespaceList.map((ns) => (
        <Card key={ns}>
          <div className="flex items-center gap-3">
            <Box className="w-5 h-5 text-[var(--color-info)]" />
            <span className="font-medium text-text">{ns}</span>
          </div>
        </Card>
      ))}
    </div>
  )
}

// Logs Tab (shared across all device types)
function LogsTab({ device }: { device: Device }) {
  const [refreshKey, setRefreshKey] = useState(0)

  const { data: logsData, isLoading } = useQuery({
    queryKey: ['logs', { source: 'device', device: device.name }, refreshKey],
    queryFn: () => logsApi.query({ source: 'device', device: device.name, limit: 50 }),
    enabled: !!device.name,
  })

  const logs = logsData?.data || []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm" style={{ color: 'var(--color-text-muted)' }}>{logs.length} 条日志</span>
        <button
          onClick={() => setRefreshKey(k => k + 1)}
          className="flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg border"
          style={{ backgroundColor: 'var(--color-surface)', borderColor: 'var(--color-border)' }}
        >
          <RefreshCw className="w-4 h-4" />
          刷新
        </button>
      </div>

      {isLoading ? (
        <div style={{ color: 'var(--color-text-muted)' }}>加载中...</div>
      ) : logs.length === 0 ? (
        <div className="text-center py-8" style={{ color: 'var(--color-text-muted)' }}>
          暂无日志
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
                <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>时间</th>
                <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>级别</th>
                <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>来源</th>
                <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>消息</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr
                  key={log.id}
                  className="hover:bg-surface-elevated transition-colors"
                  style={{ borderBottom: '1px solid var(--color-border-subtle)' }}
                >
                  <td className="py-2 px-3 font-mono whitespace-nowrap" style={{ color: 'var(--color-text-muted)', fontSize: '12px' }}>
                    {formatLogTime(log.timestamp)}
                  </td>
                  <td className="py-2 px-3">
                    <LogLevelBadge level={log.level} />
                  </td>
                  <td className="py-2 px-3" style={{ color: 'var(--color-text-secondary)' }}>
                    {log.source}
                  </td>
                  <td className="py-2 px-3 font-mono" style={{ color: 'var(--color-text-primary)', maxWidth: '400px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {log.message}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// Helper functions for logs tab
function getLogLevelStyle(level: string) {
  const configs = {
    error: { color: 'var(--color-error)', bg: 'rgba(239,68,68,0.15)' },
    warn: { color: 'var(--color-warning)', bg: 'rgba(245,158,11,0.15)' },
    info: { color: 'var(--color-info)', bg: 'rgba(59,130,246,0.15)' },
    debug: { color: 'var(--color-text-muted)', bg: 'var(--color-surface-elevated)' },
  }
  const l = level.toLowerCase()
  if (l === 'warning') return configs.warn
  if (l === 'error') return configs.error
  if (l === 'info') return configs.info
  return configs.debug
}

function formatLogTime(timestamp: string) {
  const d = new Date(timestamp)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function LogLevelBadge({ level }: { level: string }) {
  const config = getLogLevelStyle(level)
  const icons = { error: AlertCircle, warn: AlertTriangle, info: Info, debug: Bug }
  const Icon = icons[level.toLowerCase() as keyof typeof icons] || Bug
  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold whitespace-nowrap"
      style={{ backgroundColor: config.bg, color: config.color }}
    >
      <Icon className="w-3 h-3" />
      {level.toUpperCase()}
    </span>
  )
}

// Overview Tab for physical hosts (basic info)
function HostOverviewTab({ device }: { device: Device }) {
  const envColors: Record<string, { bg: string, color: string }> = {
    prod: { bg: 'var(--color-error-muted)', color: 'var(--color-error)' },
    test: { bg: 'var(--color-warning-muted)', color: 'var(--color-warning)' },
    dev: { bg: 'var(--color-info-muted)', color: 'var(--color-info)' },
  }
  const envStyle = envColors[device.environment?.toLowerCase()] || { bg: 'var(--color-info-muted)', color: 'var(--color-info)' }

  return (
    <div className="space-y-4">
      <Card>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <div className="text-xs text-text-muted uppercase tracking-wider mb-1">IP Address</div>
            <div className="font-mono text-sm">{device.labels?.ip || 'N/A'}</div>
          </div>
          <div>
            <div className="text-xs text-text-muted uppercase tracking-wider mb-1">Location</div>
            <div className="text-sm">{device.labels?.location || 'N/A'}</div>
          </div>
          <div>
            <div className="text-xs text-text-muted uppercase tracking-wider mb-1">Environment</div>
            <span
              className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
              style={{ background: envStyle.bg, color: envStyle.color }}
            >
              {device.environment}
            </span>
          </div>
          <div>
            <div className="text-xs text-text-muted uppercase tracking-wider mb-1">Registered</div>
            <div className="text-sm">{formatTime(device.registeredAt)}</div>
          </div>
        </div>
      </Card>

      {device.labels && Object.keys(device.labels).length > 0 && (
        <Card>
          <h3 className="text-sm font-semibold text-text mb-3">Labels</h3>
          <div className="flex flex-wrap gap-2">
            {Object.entries(device.labels).map(([key, value]) => (
              <span key={key} className="px-2 py-1 bg-surface-elevated rounded text-xs">
                {key}: <span className="text-text">{value}</span>
              </span>
            ))}
          </div>
        </Card>
      )}
    </div>
  )
}

// State Control Tab for physical hosts
function StateControlTab({ device }: { device: Device }) {
  const queryClient = useQueryClient()
  const [transitioning, setTransitioning] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const showToast = (message: string, type: 'success' | 'error') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 3000)
  }

  const transitionMutation = useMutation({
    mutationFn: ({ state, triggeredBy, reason }: { state: string; triggeredBy: string; reason?: string }) =>
      devicesApi.transitionState(device.id, { state, triggered_by: triggeredBy, reason }),
    onSuccess: (updatedDevice) => {
      queryClient.invalidateQueries({ queryKey: ['device', device.id] })
      showToast(`State changed to ${updatedDevice.status}`, 'success')
      setTransitioning(false)
    },
    onError: (error: Error) => {
      showToast(error.message || 'Failed to transition state', 'error')
      setTransitioning(false)
    },
  })

  const stateTransitions: Record<string, { nextState: string; label: string; icon: React.ReactNode; color: string }[]> = {
    pending: [
      { nextState: 'authenticated', label: 'Authenticate', icon: <CheckCircle className="w-4 h-4" />, color: 'var(--color-info)' },
    ],
    authenticated: [
      { nextState: 'registered', label: 'Register', icon: <Play className="w-4 h-4" />, color: 'var(--color-info)' },
    ],
    registered: [
      { nextState: 'active', label: 'Activate', icon: <CheckCircle className="w-4 h-4" />, color: 'var(--color-success)' },
    ],
    active: [
      { nextState: 'maintenance', label: 'Maintenance', icon: <Wrench className="w-4 h-4" />, color: 'var(--color-warning)' },
    ],
    maintenance: [
      { nextState: 'active', label: 'Resume', icon: <Play className="w-4 h-4" />, color: 'var(--color-success)' },
    ],
  }

  const availableTransitions = stateTransitions[device.status] || []

  const handleStateTransition = (nextState: string) => {
    setTransitioning(true)
    transitionMutation.mutate({
      state: nextState,
      triggeredBy: 'admin-ui',
      reason: `State transition via UI: ${device.status} -> ${nextState}`,
    })
  }

  return (
    <div className="space-y-4">
      {toast && (
        <div
          className="fixed top-4 right-4 z-50 px-4 py-3 rounded-lg shadow-lg text-sm font-medium animate-fade-in"
          style={{
            background: toast.type === 'success' ? 'var(--color-success)' : 'var(--color-error)',
            color: 'white',
          }}
        >
          {toast.message}
        </div>
      )}

      <Card>
        <div className="flex items-center justify-between p-3 bg-surface rounded-lg">
          <span className="text-sm text-text-secondary">Current State</span>
          <div className="flex items-center gap-2">
            {statusIcon(device.status)}
            <span className="text-sm font-medium text-text capitalize">{device.status}</span>
          </div>
        </div>

        {availableTransitions.length > 0 && (
          <div className="pt-4 border-t border-border-subtle">
            <p className="text-xs text-text-muted mb-3">Available Actions:</p>
            <div className="flex flex-wrap gap-2">
              {availableTransitions.map((transition) => (
                <button
                  key={transition.nextState}
                  onClick={() => handleStateTransition(transition.nextState)}
                  disabled={transitioning}
                  className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg transition-all disabled:opacity-50"
                  style={{
                    background: transition.color,
                    color: 'white',
                  }}
                >
                  {transition.icon}
                  {transition.label}
                  {transitioning && (
                    <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  )}
                </button>
              ))}
            </div>
          </div>
        )}

        {availableTransitions.length === 0 && (
          <p className="text-xs text-text-muted text-center pt-4">No state transitions available</p>
        )}
      </Card>
    </div>
  )
}

export function DeviceDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState('overview')
  const [refreshKey, setRefreshKey] = useState(0)

  const { data: device, isLoading: deviceLoading } = useQuery({
    queryKey: ['device', id, refreshKey],
    queryFn: () => devicesApi.get(id!),
    enabled: !!id,
  })

  if (deviceLoading) {
    return (
      <PageContainer title="Device Details" description={`Device ID: ${id}`}>
        <div className="space-y-6">
          <div className="h-48 bg-surface rounded-xl animate-pulse" />
          <div className="h-96 bg-surface rounded-xl animate-pulse" />
        </div>
      </PageContainer>
    )
  }

  if (!device) {
    return (
      <PageContainer title="Device Details" description={`Device ID: ${id}`}>
        <Card>
          <div className="flex flex-col items-center justify-center py-16 text-text-secondary">
            <div className="p-4 bg-surface-elevated rounded-full mb-4">
              <AlertCircle className="w-8 h-8 text-text-muted" />
            </div>
            <p className="font-medium">Device not found</p>
            <button
              onClick={() => navigate('/devices')}
              className="mt-4 px-4 py-2 text-sm bg-surface border border-border rounded-lg hover:bg-surface-elevated"
            >
              Back to Devices
            </button>
          </div>
        </Card>
      </PageContainer>
    )
  }

  const tabs = getTabsForDeviceType(device.type)

  // Render tab content
  const renderTabContent = () => {
    switch (activeTab) {
      case 'overview':
        return device.type === 'k8s_cluster'
          ? <K8sOverviewTab device={device} />
          : device.type === 'physical_host'
            ? <HostOverviewTab device={device} />
            : <div className="text-text-muted">No overview available</div>
      case 'nodes':
        return <K8sNodesTab device={device} />
      case 'pods':
        return <K8sPodsTab device={device} />
      case 'namespaces':
        return <K8sNamespacesTab device={device} />
      case 'logs':
        return <LogsTab device={device} />
      case 'state':
        return <StateControlTab device={device} />
      default:
        return null
    }
  }

  return (
    <PageContainer
      title={device.name}
      description={`Type: ${device.type} | Environment: ${device.environment}`}
      actions={
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/devices')}
            className="flex items-center gap-2 px-3 py-1.5 text-sm text-text-secondary hover:text-text"
          >
            <ArrowLeft className="w-4 h-4" />
            Back
          </button>
          <button
            onClick={() => setRefreshKey(k => k + 1)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-surface border border-border rounded-lg hover:bg-surface-elevated"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Device Header */}
        <Card>
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
              <div className="p-4 bg-surface-elevated rounded-xl">
                <Server className="w-8 h-8 text-[var(--color-primary)]" />
              </div>
              <div>
                <h2 className="text-xl font-semibold text-text">{device.name}</h2>
                <p className="text-sm text-text-secondary mt-0.5">{device.type}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg" style={{ background: 'var(--color-surface-elevated)' }}>
                {statusIcon(device.status)}
                <Badge variant={statusVariant(device.status)}>{device.status}</Badge>
              </div>
            </div>
          </div>
        </Card>

        {/* Tabs */}
        <div className="border-b border-border">
          <nav className="flex gap-4">
            {tabs.map(tab => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`pb-3 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab.key
                    ? 'border-[var(--color-primary)] text-[var(--color-primary)]'
                    : 'border-transparent text-text-secondary hover:text-text'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        {/* Tab Content */}
        <div>{renderTabContent()}</div>
      </div>
    </PageContainer>
  )
}
