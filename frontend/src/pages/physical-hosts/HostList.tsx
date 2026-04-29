import { useQuery } from '@tanstack/react-query'
import { Server, Activity } from 'lucide-react'
import { apiClient } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'

interface PhysicalHost {
  id: string
  hostname: string
  ip: string
  state: string
  lastAgentUpdate?: string
  labels?: Record<string, string>
}

interface HostListResponse {
  hosts: PhysicalHost[]
  total: number
}

const stateVariant = (state: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (state.toLowerCase()) {
    case 'online':
    case 'active':
      return 'success'
    case 'offline':
    case 'inactive':
      return 'error'
    case 'pending':
      return 'warning'
    default:
      return 'default'
  }
}

const formatLastUpdate = (lastUpdate?: string) => {
  if (!lastUpdate) return 'Never'
  const date = new Date(lastUpdate)
  return date.toLocaleString()
}

export function HostList() {
  const { data, isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: () => apiClient.get<HostListResponse>('/api/physical-hosts'),
  })

  const hosts = data?.hosts ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text">Physical Hosts</h1>
      </div>

      {isLoading ? (
        <p className="text-text-secondary">Loading hosts...</p>
      ) : hosts.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <Server className="w-12 h-12 mb-4 opacity-50" />
            <p>No physical hosts found</p>
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
                    <p className="text-sm text-text-secondary">{host.ip}</p>
                  </div>
                </div>
                <Badge variant={stateVariant(host.state)}>
                  {host.state}
                </Badge>
              </div>
              <div className="flex items-center gap-2 text-sm text-text-secondary">
                <Activity className="w-4 h-4" />
                <span>Last update: {formatLastUpdate(host.lastAgentUpdate)}</span>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}