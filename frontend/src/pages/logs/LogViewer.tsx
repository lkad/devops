import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, RefreshCw, AlertCircle } from 'lucide-react'
import { logsApi } from '@/api/endpoints/logs'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'

const levelVariant = (level: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (level.toLowerCase()) {
    case 'error':
      return 'error'
    case 'warn':
      return 'warning'
    case 'info':
      return 'info'
    case 'debug':
      return 'default'
    default:
      return 'default'
  }
}

const formatTimestamp = (timestamp: string) => {
  const date = new Date(timestamp)
  return date.toLocaleString()
}

export function LogViewer() {
  const [searchQuery, setSearchQuery] = useState('')
  const [levelFilter, setLevelFilter] = useState('')

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['logs'],
    queryFn: () => logsApi.query({ limit: 100 }),
  })

  const logs = data?.logs ?? []

  const filteredLogs = useMemo(() => {
    return logs.filter(log => {
      const matchesSearch = !searchQuery ||
        log.message.toLowerCase().includes(searchQuery.toLowerCase()) ||
        log.source.toLowerCase().includes(searchQuery.toLowerCase())
      const matchesLevel = !levelFilter || log.level === levelFilter
      return matchesSearch && matchesLevel
    })
  }, [logs, searchQuery, levelFilter])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text">Log Viewer</h1>
        <Button variant="secondary" onClick={() => refetch()}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      </div>

      <Card>
        <div className="flex flex-col sm:flex-row gap-4 mb-6">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-text-muted" />
            <Input
              placeholder="Search logs..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>
          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value)}
            className="px-3 py-2 rounded-md bg-surface border border-border text-text-primary focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent"
          >
            <option value="">All Levels</option>
            <option value="error">Error</option>
            <option value="warn">Warn</option>
            <option value="info">Info</option>
            <option value="debug">Debug</option>
          </select>
        </div>

        {isLoading ? (
          <p className="text-text-secondary">Loading logs...</p>
        ) : filteredLogs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
            <p>No logs found</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full font-mono text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Timestamp</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Level</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Source</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-text-secondary">Message</th>
                </tr>
              </thead>
              <tbody>
                {filteredLogs.map(log => (
                  <tr key={log.id} className="border-b border-border last:border-b-0 hover:bg-surface-elevated">
                    <td className="py-3 px-4 text-text-secondary whitespace-nowrap">
                      {formatTimestamp(log.timestamp)}
                    </td>
                    <td className="py-3 px-4">
                      <Badge variant={levelVariant(log.level)}>
                        {log.level.toUpperCase()}
                      </Badge>
                    </td>
                    <td className="py-3 px-4 text-text-secondary">{log.source}</td>
                    <td className="py-3 px-4 text-text">{log.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}