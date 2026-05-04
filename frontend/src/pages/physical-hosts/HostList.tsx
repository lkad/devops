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
        <span className="text-sm text-text-muted">{hosts.length} hosts</span>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-16 bg-surface rounded-lg animate-pulse" />
          ))}
        </div>
      ) : hosts.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center justify-center py-16 text-text-secondary">
            <div className="p-4 bg-surface-elevated rounded-full mb-4">
              <Server className="w-8 h-8 text-text-muted" />
            </div>
            <p className="font-medium">No physical hosts found</p>
            <p className="text-sm mt-1 text-text-muted">Register hosts via containerlab or API</p>
          </div>
        </Card>
      ) : (
        <Card className="overflow-hidden" style={{ padding: 0 }}>
          <table className="w-full">
            <thead>
              <tr className="border-b border-border-subtle" style={{ background: 'var(--color-surface-elevated)' }}>
                <th className="text-left py-3 px-5 text-xs font-semibold text-text-secondary uppercase tracking-wider">Host</th>
                <th className="text-left py-3 px-5 text-xs font-semibold text-text-secondary uppercase tracking-wider">Status</th>
                <th className="text-left py-3 px-5 text-xs font-semibold text-text-secondary uppercase tracking-wider">Location</th>
                <th className="text-left py-3 px-5 text-xs font-semibold text-text-secondary uppercase tracking-wider">Environment</th>
                <th className="text-left py-3 px-5 text-xs font-semibold text-text-secondary uppercase tracking-wider">Registered</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map((host, idx) => (
                <tr
                  key={host.id}
                  className="border-b border-border-subtle last:border-b-0 hover:bg-surface-elevated cursor-pointer transition-colors"
                  style={{ animationDelay: `${idx * 50}ms` }}
                  onClick={() => navigate(`/physical-hosts/${host.id}`)}
                >
                  <td className="py-4 px-5">
                    <div className="flex items-center gap-3">
                      <div className="p-2 bg-surface-elevated rounded-lg">
                        <Server className="w-4 h-4 text-text-secondary" />
                      </div>
                      <div>
                        <span className="font-medium text-text">{host.name}</span>
                        <span className="ml-2 text-xs text-text-muted font-mono">{host.id.slice(0, 12)}...</span>
                      </div>
                    </div>
                  </td>
                  <td className="py-4 px-5">
                    <Badge variant={stateVariant(host.status)}>{host.status}</Badge>
                  </td>
                  <td className="py-4 px-5 text-sm text-text-secondary">{host.labels?.location || 'N/A'}</td>
                  <td className="py-4 px-5">
                    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                      style={{
                        background: host.environment === 'prod' ? 'var(--color-error-muted)' :
                          host.environment === 'test' ? 'var(--color-warning-muted)' :
                            'var(--color-info-muted)',
                        color: host.environment === 'prod' ? 'var(--color-error)' :
                          host.environment === 'test' ? 'var(--color-warning)' :
                            'var(--color-info)'
                      }}>
                      {host.environment || 'N/A'}
                    </span>
                  </td>
                  <td className="py-4 px-5 text-sm text-text-muted">{formatTime(host.registeredAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  )
}