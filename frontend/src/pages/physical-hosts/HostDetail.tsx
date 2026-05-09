import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Server, AlertCircle, RefreshCw, Activity, ArrowLeft, Cpu, HardDrive, Network, CheckCircle, XCircle, Pause, Play, Wrench, FolderOpen } from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi } from '@/api/endpoints/devices'
import { logsApi } from '@/api/endpoints/logs'
import { projectApi, projectsApi } from '@/api/endpoints/projects'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
      return 'success'
    case 'pending':
      return 'warning'
    case 'authenticated':
    case 'registered':
      return 'info'
    case 'inactive':
    case 'offline':
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
      return <CheckCircle className="w-4 h-4 text-[var(--color-success)]" />
    case 'pending':
      return <Pause className="w-4 h-4 text-[var(--color-warning)]" />
    case 'authenticated':
    case 'registered':
      return <Play className="w-4 h-4 text-[var(--color-info)]" />
    case 'inactive':
    case 'offline':
      return <XCircle className="w-4 h-4 text-[var(--color-error)]" />
    case 'maintenance':
      return <Wrench className="w-4 h-4 text-[var(--color-warning)]" />
    default:
      return null
  }
}

const getLevelColor = (level: string) => {
  switch (level) {
    case 'error': return { bg: 'var(--color-error-muted)', text: 'var(--color-error)', dot: 'var(--color-error)' }
    case 'warn': return { bg: 'var(--color-warning-muted)', text: 'var(--color-warning)', dot: 'var(--color-warning)' }
    default: return { bg: 'var(--color-success-muted)', text: 'var(--color-success)', dot: 'var(--color-success)' }
  }
}

const formatTime = (timestamp?: string) => {
  if (!timestamp) return 'Never'
  return new Date(timestamp).toLocaleString()
}

// State machine for physical hosts
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

type TabType = 'overview' | 'logs' | 'projects'

export function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<TabType>('overview')
  const [refreshKey, setRefreshKey] = useState(0)
  const [transitioning, setTransitioning] = useState(false)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const { data: device, isLoading: deviceLoading } = useQuery({
    queryKey: ['devices', id],
    queryFn: () => devicesApi.get(id!),
    enabled: !!id,
  })

  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['device-logs', device?.name, refreshKey],
    queryFn: () => logsApi.query({
      source: 'physical_host',
      device: device?.name,
      limit: 30,
    }),
    enabled: !!device?.name && activeTab === 'logs',
  })

  const { data: linkedProjects, isLoading: projectsLoading } = useQuery({
    queryKey: ['physical-host', id, 'projects'],
    queryFn: () => projectApi.getProjectsForPhysicalHost(id!),
    enabled: activeTab === 'projects' && !!id,
  })

  const unlinkMutation = useMutation({
    mutationFn: (projectId: string) => projectsApi.unlinkResource(projectId, 'physical_host', id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['physical-host', id, 'projects'] })
      showToast('已取消关联', 'success')
    },
    onError: (error: Error) => {
      showToast(error.message || '取消关联失败', 'error')
    },
  })

  const logs = logsData?.data ?? []

  const transitionMutation = useMutation({
    mutationFn: ({ state, triggeredBy, reason }: { state: string; triggeredBy: string; reason?: string }) =>
      devicesApi.transitionState(id!, { state, triggered_by: triggeredBy, reason }),
    onSuccess: (updatedDevice) => {
      queryClient.invalidateQueries({ queryKey: ['devices', id] })
      showToast(`State changed to ${updatedDevice.status}`, 'success')
      setTransitioning(false)
    },
    onError: (error: Error) => {
      showToast(error.message || 'Failed to transition state', 'error')
      setTransitioning(false)
    },
  })

  const showToast = (message: string, type: 'success' | 'error') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 3000)
  }

  const handleStateTransition = (nextState: string) => {
    if (!device) return
    setTransitioning(true)
    transitionMutation.mutate({
      state: nextState,
      triggeredBy: 'admin-ui',
      reason: `State transition via UI: ${device.status} -> ${nextState}`,
    })
  }

  const availableTransitions = device ? stateTransitions[device.status] || [] : []

  if (deviceLoading) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <div className="space-y-6">
          <div className="h-48 bg-surface rounded-xl animate-pulse" />
          <div className="h-96 bg-surface rounded-xl animate-pulse" />
        </div>
      </PageContainer>
    )
  }

  if (!device) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <Card>
          <div className="flex flex-col items-center justify-center py-16 text-text-secondary">
            <div className="p-4 bg-surface-elevated rounded-full mb-4">
              <AlertCircle className="w-8 h-8 text-text-muted" />
            </div>
            <p className="font-medium">Host not found</p>
            <p className="text-sm mt-1 text-text-muted">The requested host does not exist</p>
            <button
              onClick={() => navigate('/physical-hosts')}
              className="mt-4 px-4 py-2 text-sm bg-surface border border-border rounded-lg hover:bg-surface-elevated transition-colors"
            >
              Back to Hosts
            </button>
          </div>
        </Card>
      </PageContainer>
    )
  }

  const envColors: Record<string, { bg: string, color: string }> = {
    prod: { bg: 'var(--color-error-muted)', color: 'var(--color-error)' },
    test: { bg: 'var(--color-warning-muted)', color: 'var(--color-warning)' },
    dev: { bg: 'var(--color-info-muted)', color: 'var(--color-info)' },
  }
  const envStyle = envColors[device.environment?.toLowerCase() || ''] || { bg: 'var(--color-info-muted)', color: 'var(--color-info)' }

  return (
    <PageContainer
      title={device.name}
      description={`Type: ${device.type} | Environment: ${device.environment}`}
      actions={
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/physical-hosts')}
            className="flex items-center gap-2 px-3 py-1.5 text-sm text-text-secondary hover:text-text transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            Back
          </button>
          <button
            onClick={() => setRefreshKey(k => k + 1)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-surface border border-border rounded-lg hover:bg-surface-elevated transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
      }
    >
      {/* Toast notification */}
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

      {/* Tab navigation */}
      <div className="flex items-center gap-1 mb-6 bg-surface rounded-lg p-1">
        <button
          onClick={() => setActiveTab('overview')}
          className={`px-4 py-2 text-sm font-medium rounded-md transition-colors ${
            activeTab === 'overview'
              ? 'bg-surface-elevated text-text shadow-sm'
              : 'text-text-secondary hover:text-text'
          }`}
        >
          Overview
        </button>
        <button
          onClick={() => setActiveTab('logs')}
          className={`px-4 py-2 text-sm font-medium rounded-md transition-colors ${
            activeTab === 'logs'
              ? 'bg-surface-elevated text-text shadow-sm'
              : 'text-text-secondary hover:text-text'
          }`}
        >
          Logs
        </button>
        <button
          onClick={() => setActiveTab('projects')}
          className={`px-4 py-2 text-sm font-medium rounded-md transition-colors ${
            activeTab === 'projects'
              ? 'bg-surface-elevated text-text shadow-sm'
              : 'text-text-secondary hover:text-text'
          }`}
        >
          关联项目
        </button>
      </div>

      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
          {/* Left column - Main content (8 cols) */}
          <div className="lg:col-span-8 space-y-6">
            {/* Host Info Card */}
            <Card>
              <div className="flex items-start justify-between mb-6">
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
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium" style={{ background: envStyle.bg, color: envStyle.color }}>
                    {device.environment}
                  </span>
                </div>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
                <div className="p-4 bg-surface rounded-lg">
                  <div className="flex items-center gap-2 text-text-muted mb-2">
                    <Network className="w-4 h-4" />
                    <span className="text-xs uppercase tracking-wider">Location</span>
                  </div>
                  <p className="text-sm font-medium text-text">{device.labels?.location || 'N/A'}</p>
                </div>
                <div className="p-4 bg-surface rounded-lg">
                  <div className="flex items-center gap-2 text-text-muted mb-2">
                    <Cpu className="w-4 h-4" />
                    <span className="text-xs uppercase tracking-wider">IP Address</span>
                  </div>
                  <p className="text-sm font-mono text-text">{device.labels?.ip || 'N/A'}</p>
                </div>
                <div className="p-4 bg-surface rounded-lg">
                  <div className="flex items-center gap-2 text-text-muted mb-2">
                    <HardDrive className="w-4 h-4" />
                    <span className="text-xs uppercase tracking-wider">Registered</span>
                  </div>
                  <p className="text-sm font-medium text-text">{formatTime(device.registeredAt)}</p>
                </div>
                <div className="p-4 bg-surface rounded-lg">
                  <div className="flex items-center gap-2 text-text-muted mb-2">
                    <Activity className="w-4 h-4" />
                    <span className="text-xs uppercase tracking-wider">Source</span>
                  </div>
                  <p className="text-sm font-medium text-text">{device.labels?.source || 'N/A'}</p>
                </div>
              </div>
            </Card>

            {/* Quick Stats */}
            <Card>
              <h3 className="text-sm font-semibold text-text mb-4">Quick Stats</h3>
              <div className="space-y-3">
                <div className="flex items-center justify-between p-3 bg-surface rounded-lg">
                  <span className="text-sm text-text-secondary">Total Logs</span>
                  <span className="text-lg font-semibold text-[var(--color-primary)]">{logs.length}</span>
                </div>
                <div className="flex items-center justify-between p-3 bg-surface rounded-lg">
                  <span className="text-sm text-text-secondary">Device Type</span>
                  <span className="text-sm font-medium text-text">{device.type}</span>
                </div>
                <div className="flex items-center justify-between p-3 bg-surface rounded-lg">
                  <span className="text-sm text-text-secondary">Environment</span>
                  <span className="text-sm font-medium" style={{ color: envStyle.color }}>{device.environment}</span>
                </div>
              </div>
            </Card>
          </div>

          {/* Right column - Sidebar (4 cols) */}
          <div className="lg:col-span-4 space-y-6">
            {/* State Control */}
            <Card>
              <h3 className="text-sm font-semibold text-text mb-4">State Control</h3>
              <div className="space-y-3">
                <div className="flex items-center justify-between p-3 bg-surface rounded-lg">
                  <span className="text-sm text-text-secondary">Current State</span>
                  <div className="flex items-center gap-2">
                    {statusIcon(device.status)}
                    <span className="text-sm font-medium text-text capitalize">{device.status}</span>
                  </div>
                </div>

                {availableTransitions.length > 0 && (
                  <div className="pt-2 border-t border-border-subtle">
                    <p className="text-xs text-text-muted mb-2">Available Actions:</p>
                    <div className="space-y-2">
                      {availableTransitions.map((transition) => (
                        <button
                          key={transition.nextState}
                          onClick={() => handleStateTransition(transition.nextState)}
                          disabled={transitioning}
                          className="w-full flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg transition-all disabled:opacity-50"
                          style={{
                            background: transition.color,
                            color: 'white',
                          }}
                        >
                          {transition.icon}
                          {transition.label}
                          {transitioning && <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {availableTransitions.length === 0 && (
                  <p className="text-xs text-text-muted text-center pt-2">No state transitions available</p>
                )}
              </div>
            </Card>

            {/* Labels */}
            <Card>
              <h3 className="text-sm font-semibold text-text mb-4">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(device.labels || {}).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium bg-surface-elevated text-text-secondary"
                  >
                    {key}: <span className="ml-1 text-text">{value}</span>
                  </span>
                ))}
                {(!device.labels || Object.keys(device.labels).length === 0) && (
                  <span className="text-sm text-text-muted">No labels</span>
                )}
              </div>
            </Card>

            {/* Actions */}
            <Card>
              <h3 className="text-sm font-semibold text-text mb-4">Actions</h3>
              <div className="space-y-2">
                <button
                  onClick={() => setRefreshKey(k => k + 1)}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium bg-surface-elevated border border-border rounded-lg hover:bg-surface transition-colors"
                >
                  <RefreshCw className="w-4 h-4" />
                  Refresh Logs
                </button>
              </div>
            </Card>
          </div>
        </div>
      )}

      {activeTab === 'logs' && (
        <div className="space-y-6">
          {/* Logs Card */}
          <Card>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-surface-elevated rounded-lg">
                  <Activity className="w-4 h-4 text-text-secondary" />
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-text">Recent Logs</h3>
                  <p className="text-xs text-text-muted">{logs.length} entries</p>
                </div>
              </div>
              {logsLoading && (
                <div className="flex items-center gap-2 text-xs text-text-muted">
                  <div className="w-4 h-4 border-2 border-text-muted border-t-transparent rounded-full animate-spin" />
                  Loading...
                </div>
              )}
            </div>

            {logs.length === 0 && !logsLoading ? (
              <div className="text-center py-12 text-text-secondary">
                <div className="p-4 bg-surface-elevated rounded-full inline-block mb-4">
                  <Activity className="w-6 h-6 text-text-muted" />
                </div>
                <p className="font-medium">No logs available</p>
                <p className="text-sm mt-1 text-text-muted">Logs are collected from Loki when available</p>
              </div>
            ) : (
              <div className="space-y-2 max-h-[600px] overflow-y-auto">
                {logs.map((log, idx) => {
                  const colors = getLevelColor(log.level)
                  return (
                    <div
                      key={log.id || idx}
                      className="flex items-start gap-3 p-3 rounded-lg hover:bg-surface-elevated transition-colors"
                    >
                      <div className="flex-shrink-0 mt-0.5">
                        <span
                          className="inline-block w-2 h-2 rounded-full"
                          style={{ background: colors.dot }}
                        />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between gap-3 mb-1">
                          <span
                            className="text-xs font-semibold uppercase tracking-wider"
                            style={{ color: colors.text }}
                          >
                            {log.level}
                          </span>
                          <span className="text-xs text-text-muted">
                            {new Date(log.timestamp).toLocaleString()}
                          </span>
                        </div>
                        <p className="text-sm text-text leading-relaxed">{log.message}</p>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>
        </div>
      )}

      {activeTab === 'projects' && (
        <div className="space-y-6">
          {/* Linked Projects Card */}
          <Card>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-surface-elevated rounded-lg">
                  <FolderOpen className="w-4 h-4 text-text-secondary" />
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-text">关联项目</h3>
                  <p className="text-xs text-text-muted">{linkedProjects?.length || 0} 个项目</p>
                </div>
              </div>
              {projectsLoading && (
                <div className="flex items-center gap-2 text-xs text-text-muted">
                  <div className="w-4 h-4 border-2 border-text-muted border-t-transparent rounded-full animate-spin" />
                  Loading...
                </div>
              )}
            </div>

            {projectsLoading ? (
              <div className="text-center py-12 text-text-secondary">
                <div className="w-8 h-8 border-2 border-text-muted border-t-transparent rounded-full animate-spin mx-auto mb-4" />
                <p className="text-sm">加载中...</p>
              </div>
            ) : linkedProjects && linkedProjects.length > 0 ? (
              <div className="space-y-3">
                {linkedProjects.map((project) => (
                  <div
                    key={project.id}
                    className="flex items-center justify-between p-4 bg-surface rounded-lg hover:bg-surface-elevated transition-colors group"
                  >
                    <div className="flex items-center gap-3 flex-1">
                      <div className="p-2 bg-surface-elevated rounded-lg group-hover:bg-primary/10">
                        <FolderOpen className="w-4 h-4 text-text-secondary group-hover:text-primary" />
                      </div>
                      <Link to={`/projects/${project.id}`} className="flex-1">
                        <p className="text-sm font-medium text-text">{project.name}</p>
                        <p className="text-xs text-text-muted mt-0.5">
                          {project.type === 'frontend' ? '前端项目' : '后端项目'}
                        </p>
                      </Link>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-xs text-text-muted">
                        创建于 {new Date(project.createdAt).toLocaleDateString()}
                      </span>
                      <button
                        onClick={(e) => {
                          e.preventDefault()
                          e.stopPropagation()
                          if (confirm('确定要取消关联此项目吗？')) {
                            unlinkMutation.mutate(project.id)
                          }
                        }}
                        disabled={unlinkMutation.isPending}
                        className="px-3 py-1.5 text-xs font-medium text-error bg-error/10 hover:bg-error/20 rounded-md transition-colors disabled:opacity-50"
                      >
                        {unlinkMutation.isPending ? '处理中...' : '取消关联'}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-12 text-text-secondary">
                <div className="p-4 bg-surface-elevated rounded-full inline-block mb-4">
                  <FolderOpen className="w-6 h-6 text-text-muted" />
                </div>
                <p className="font-medium">该主机未关联任何项目</p>
                <p className="text-sm mt-1 text-text-muted">从项目详情页可以将此主机添加到项目</p>
              </div>
            )}
          </Card>
        </div>
      )}
    </PageContainer>
  )
}