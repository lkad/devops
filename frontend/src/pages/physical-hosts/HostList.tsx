import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Server } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi } from '@/api/endpoints/devices'

const stateVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
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

const formatTime = (timestamp?: string) => {
  if (!timestamp) return 'Never'
  return new Date(timestamp).toLocaleString()
}

export function HostList() {
  const navigate = useNavigate()

  const { data, isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: () => devicesApi.list({ type: 'physical_host' }),
  })

  const hosts = data?.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text">Physical Hosts</h1>
        <span className="text-sm text-text-secondary">{hosts.length} hosts</span>
      </div>

      {isLoading ? (
        <p className="text-text-secondary">Loading hosts...</p>
      ) : hosts.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <Server className="w-12 h-12 mb-4 opacity-50" />
            <p>No physical hosts found</p>
            <p className="text-sm mt-1">Register hosts via containerlab or API</p>
          </div>
        </Card>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border">
                <th className="text-left py-3 px-4 text-xs font-medium text-text-muted uppercase tracking-wide">Host</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-muted uppercase tracking-wide">Status</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-muted uppercase tracking-wide">Type</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-muted uppercase tracking-wide">Location</th>
                <th className="text-left py-3 px-4 text-xs font-medium text-text-muted uppercase tracking-wide">Registered</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map(host => (
                <tr
                  key={host.id}
                  className="border-b border-border/50 hover:bg-surface/50 cursor-pointer transition-colors"
                  onClick={() => navigate(`/physical-hosts/${host.id}`)}
                >
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-3">
                      <div className="p-2 bg-surface rounded-lg">
                        <Server className="w-5 h-5 text-text-secondary" />
                      </div>
                      <span className="font-medium text-text">{host.name}</span>
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <Badge variant={stateVariant(host.status)}>{host.status}</Badge>
                  </td>
                  <td className="py-3 px-4 text-sm text-text-secondary">{host.type}</td>
                  <td className="py-3 px-4 text-sm text-text-secondary">{host.labels?.location || 'N/A'}</td>
                  <td className="py-3 px-4 text-sm text-text-secondary">{formatTime(host.registeredAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}