import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Server, AlertCircle, RefreshCw, Activity, ArrowLeft, Cpu, HardDrive, Network } from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi } from '@/api/endpoints/devices'
import { logsApi } from '@/api/endpoints/logs'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
      return 'success'
    case 'pending':
      return 'warning'
    case 'inactive':
    case 'offline':
      return 'error'
    default:
      return 'default'
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

export function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [refreshKey, setRefreshKey] = useState(0)

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
    enabled: !!device?.name,
  })

  const logs = logsData?.data ?? []

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
              <div className="flex items-center gap-2">
                <Badge variant={statusVariant(device.status)}>{device.status}</Badge>
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
                  <span className="text-xs uppercase tracking-wider">Device ID</span>
                </div>
                <p className="text-sm font-mono text-text">{device.id.slice(0, 16)}...</p>
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
              <div className="space-y-2 max-h-[400px] overflow-y-auto">
                {logs.map((log, idx) => {
                  const colors = getLevelColor(log.level)
                  return (
                    <div
                      key={log.id || idx}
                      className="flex items-start gap-3 p-3 rounded-lg hover:bg-surface-elevated transition-colors"
                      style={{ animationDelay: `${idx * 30}ms` }}
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

        {/* Right column - Sidebar (4 cols) */}
        <div className="lg:col-span-4 space-y-6">
          {/* Quick Stats */}
          <Card>
            <h3 className="text-sm font-semibold text-text mb-4">Quick Stats</h3>
            <div className="space-y-4">
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
    </PageContainer>
  )
}