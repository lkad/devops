import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Server, Cpu, MemoryStick, HardDrive, Pencil, Trash2, AlertCircle } from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { HostForm } from './HostForm'
import { physicalHostsApi, type CreatePhysicalHostRequest } from '@/api/endpoints/physicalHosts'

const stateVariant = (state: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (state.toLowerCase()) {
    case 'online':
      return 'success'
    case 'monitoring_issue':
      return 'warning'
    case 'offline':
      return 'error'
    default:
      return 'default'
  }
}

const formatUptime = (uptime?: string) => {
  if (!uptime) return 'N/A'
  return uptime
}

export function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [isEditOpen, setIsEditOpen] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const { data: host, isLoading: hostLoading } = useQuery({
    queryKey: ['physical-hosts', id],
    queryFn: () => physicalHostsApi.getHost(id!),
    enabled: !!id,
  })

  const { data: metrics } = useQuery({
    queryKey: ['physical-hosts', id, 'metrics'],
    queryFn: () => physicalHostsApi.getMetrics(id!),
    enabled: !!id,
  })

  const deleteMutation = useMutation({
    mutationFn: () => physicalHostsApi.deleteHost(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['physical-hosts'] })
      navigate('/physical-hosts')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (data: CreatePhysicalHostRequest) =>
      physicalHostsApi.createHost(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['physical-hosts', id] })
      setIsEditOpen(false)
    },
  })

  if (hostLoading) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <p className="text-text-secondary">Loading host details...</p>
      </PageContainer>
    )
  }

  if (!host) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <Card>
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
            <p>Host not found</p>
          </div>
        </Card>
      </PageContainer>
    )
  }

  return (
    <PageContainer
      title={host.hostname}
      description={`Host ID: ${id}`}
      actions={
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsEditOpen(true)}
            leftIcon={<Pencil className="w-4 h-4" />}
          >
            Edit
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={() => setShowDeleteConfirm(true)}
            leftIcon={<Trash2 className="w-4 h-4" />}
          >
            Delete
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Host Info Card */}
        <Card>
          <div className="flex items-start justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className="p-3 bg-surface rounded-lg">
                <Server className="w-6 h-6 text-text-secondary" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-text">{host.hostname}</h2>
                <p className="text-sm text-text-secondary">{host.ip}:{host.port}</p>
              </div>
            </div>
            <Badge variant={stateVariant(host.state)}>
              {host.state.replace('_', ' ')}
            </Badge>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <p className="text-xs text-text-muted uppercase tracking-wide">Monitoring Status</p>
              <p className="text-sm font-medium text-text mt-1">{host.monitoringStatus || 'Unknown'}</p>
            </div>
            <div>
              <p className="text-xs text-text-muted uppercase tracking-wide">Last Heartbeat</p>
              <p className="text-sm font-medium text-text mt-1">
                {host.lastHeartbeat ? new Date(host.lastHeartbeat).toLocaleString() : 'Never'}
              </p>
            </div>
            <div>
              <p className="text-xs text-text-muted uppercase tracking-wide">Registered</p>
              <p className="text-sm font-medium text-text mt-1">
                {host.registeredAt ? new Date(host.registeredAt).toLocaleString() : 'Unknown'}
              </p>
            </div>
            <div>
              <p className="text-xs text-text-muted uppercase tracking-wide">Uptime</p>
              <p className="text-sm font-medium text-text mt-1">
                {formatUptime(metrics?.uptime?.formatted)}
              </p>
            </div>
          </div>
        </Card>

        {/* Metrics Cards */}
        {metrics && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* CPU */}
            <Card>
              <div className="flex items-center gap-2 mb-3">
                <Cpu className="w-4 h-4 text-text-secondary" />
                <h3 className="text-sm font-medium text-text">CPU</h3>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Usage</span>
                  <span className="font-medium text-text">{metrics.cpu.usage.toFixed(1)}%</span>
                </div>
                <div className="w-full bg-surface rounded-full h-2">
                  <div
                    className="bg-primary h-2 rounded-full transition-all"
                    style={{ width: `${metrics.cpu.usage}%` }}
                  />
                </div>
                <p className="text-xs text-text-muted">{metrics.cpu.cores} cores</p>
              </div>
            </Card>

            {/* Memory */}
            <Card>
              <div className="flex items-center gap-2 mb-3">
                <MemoryStick className="w-4 h-4 text-text-secondary" />
                <h3 className="text-sm font-medium text-text">Memory</h3>
              </div>
              <div className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Usage</span>
                  <span className="font-medium text-text">{metrics.memory.usagePercent.toFixed(1)}%</span>
                </div>
                <div className="w-full bg-surface rounded-full h-2">
                  <div
                    className="bg-primary h-2 rounded-full transition-all"
                    style={{ width: `${metrics.memory.usagePercent}%` }}
                  />
                </div>
                <p className="text-xs text-text-muted">
                  {formatBytes(metrics.memory.used)} / {formatBytes(metrics.memory.total)}
                </p>
              </div>
            </Card>

            {/* Disk */}
            <Card>
              <div className="flex items-center gap-2 mb-3">
                <HardDrive className="w-4 h-4 text-text-secondary" />
                <h3 className="text-sm font-medium text-text">Disk</h3>
              </div>
              <div className="space-y-2">
                {metrics.disk.disks.slice(0, 2).map((disk, idx) => {
                  const usedPercent = (disk.used / disk.total) * 100
                  return (
                    <div key={idx}>
                      <div className="flex justify-between text-sm">
                        <span className="text-text-secondary">{disk.device}</span>
                        <span className="font-medium text-text">{usedPercent.toFixed(1)}%</span>
                      </div>
                      <div className="w-full bg-surface rounded-full h-2 mt-1">
                        <div
                          className="bg-primary h-2 rounded-full transition-all"
                          style={{ width: `${usedPercent}%` }}
                        />
                      </div>
                      <p className="text-xs text-text-muted">
                        {formatBytes(disk.used)} / {formatBytes(disk.total)}
                      </p>
                    </div>
                  )
                })}
              </div>
            </Card>
          </div>
        )}
      </div>

      {/* Edit Modal */}
      <HostForm
        isOpen={isEditOpen}
        onClose={() => setIsEditOpen(false)}
        onSubmit={(data) => updateMutation.mutate(data)}
        host={host}
        isLoading={updateMutation.isPending}
      />

      {/* Delete Confirmation */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="max-w-md w-full mx-4">
            <div className="flex items-center gap-3 mb-4">
              <AlertCircle className="w-6 h-6 text-error" />
              <h3 className="text-lg font-semibold text-text">Delete Host</h3>
            </div>
            <p className="text-text-secondary mb-6">
              Are you sure you want to delete <strong>{host.hostname}</strong>? This action cannot be undone.
            </p>
            <div className="flex justify-end gap-3">
              <Button variant="secondary" onClick={() => setShowDeleteConfirm(false)}>
                Cancel
              </Button>
              <Button variant="danger" onClick={() => deleteMutation.mutate()} loading={deleteMutation.isPending}>
                Delete
              </Button>
            </div>
          </Card>
        </div>
      )}
    </PageContainer>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}