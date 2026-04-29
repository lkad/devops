import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Server, Activity, Plus } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
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

const formatLastHeartbeat = (lastHeartbeat?: string) => {
  if (!lastHeartbeat) return 'Never'
  const date = new Date(lastHeartbeat)
  return date.toLocaleString()
}

export function HostList() {
  const [isFormOpen, setIsFormOpen] = useState(false)
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: physicalHostsApi.listHosts,
  })

  const createMutation = useMutation({
    mutationFn: (data: CreatePhysicalHostRequest) => physicalHostsApi.createHost(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['physical-hosts'] })
      setIsFormOpen(false)
    },
  })

  const hosts = data?.hosts ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text">Physical Hosts</h1>
        <button
          onClick={() => setIsFormOpen(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-md hover:bg-primary-hover transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add Host
        </button>
      </div>

      {isLoading ? (
        <p className="text-text-secondary">Loading hosts...</p>
      ) : hosts.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <Server className="w-12 h-12 mb-4 opacity-50" />
            <p>No physical hosts found</p>
            <p className="text-sm mt-1">Click "Add Host" to register your first host</p>
          </div>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {hosts.map(host => (
            <Card key={host.id} className="flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-surface rounded-lg">
                    <Server className="w-5 h-5 text-text-secondary" />
                  </div>
                  <div>
                    <p className="font-medium text-text">{host.hostname}</p>
                    <p className="text-sm text-text-secondary">{host.ip}:{host.port}</p>
                  </div>
                </div>
                <Badge variant={stateVariant(host.state)}>
                  {host.state.replace('_', ' ')}
                </Badge>
              </div>
              <div className="flex items-center gap-2 text-sm text-text-secondary">
                <Activity className="w-4 h-4" />
                <span>Last heartbeat: {formatLastHeartbeat(host.lastHeartbeat)}</span>
              </div>
            </Card>
          ))}
        </div>
      )}

      <HostForm
        isOpen={isFormOpen}
        onClose={() => setIsFormOpen(false)}
        onSubmit={createMutation.mutate}
        isLoading={createMutation.isPending}
      />
    </div>
  )
}