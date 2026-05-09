import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, RefreshCw, AlertCircle, AlertTriangle, Info, Bug } from 'lucide-react'
import { logsApi } from '@/api/endpoints/logs'
import { devicesApi } from '@/api/endpoints/devices'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'

const levelConfig = {
  error: { icon: AlertCircle, color: 'var(--color-error)', bg: 'rgba(239,68,68,0.15)', label: 'ERROR' },
  warn: { icon: AlertTriangle, color: 'var(--color-warning)', bg: 'rgba(245,158,11,0.15)', label: 'WARN' },
  info: { icon: Info, color: 'var(--color-info)', bg: 'rgba(59,130,246,0.15)', label: 'INFO' },
  debug: { icon: Bug, color: 'var(--color-text-muted)', bg: 'var(--color-surface-elevated)', label: 'DEBUG' },
} as const

function getLevelStyle(level: string) {
  const l = level.toLowerCase()
  if (l === 'warning') return levelConfig.warn
  if (l === 'error') return levelConfig.error
  if (l === 'info') return levelConfig.info
  return levelConfig.debug
}

function formatTime(timestamp: string) {
  const d = new Date(timestamp)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function LevelBadge({ level }: { level: string }) {
  const config = getLevelStyle(level)
  const Icon = config.icon
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

export function LogViewer() {
  const [searchQuery, setSearchQuery] = useState('')
  const [levelFilter, setLevelFilter] = useState('')
  const [deviceFilter, setDeviceFilter] = useState('')

  const { data: devicesData } = useQuery({
    queryKey: ['devices'],
    queryFn: () => devicesApi.list(),
  })

  const devices = devicesData?.data ?? []

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['logs', { level: levelFilter, device: deviceFilter }],
    queryFn: () => logsApi.query({
      limit: 100,
      level: levelFilter || undefined,
      device: deviceFilter || undefined,
    }),
  })

  const logs = data?.data ?? []

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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold" style={{ color: 'var(--color-text-primary)' }}>日志查看器</h1>
        <Button variant="secondary" onClick={() => refetch()}>
          <RefreshCw className="w-4 h-4 mr-2" />
          刷新
        </Button>
      </div>

      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4" style={{ color: 'var(--color-text-muted)' }} />
          <Input
            placeholder="搜索日志..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
        <select
          value={levelFilter}
          onChange={(e) => setLevelFilter(e.target.value)}
          className="px-3 py-2 rounded-md bg-surface border border-border focus:outline-none focus:ring-2 focus:ring-primary"
          style={{ color: 'var(--color-text-primary)' }}
        >
          <option value="">全部级别</option>
          <option value="error">错误</option>
          <option value="warn">警告</option>
          <option value="info">信息</option>
          <option value="debug">调试</option>
        </select>
        <select
          value={deviceFilter}
          onChange={(e) => setDeviceFilter(e.target.value)}
          className="px-3 py-2 rounded-md bg-surface border border-border focus:outline-none focus:ring-2 focus:ring-primary"
          style={{ color: 'var(--color-text-primary)' }}
        >
          <option value="">全部设备</option>
          {devices.map(device => (
            <option key={device.id} value={device.name}>{device.name}</option>
          ))}
        </select>
      </div>

      <Card>
        {isLoading ? (
          <p style={{ color: 'var(--color-text-secondary)' }}>加载中...</p>
        ) : filteredLogs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12" style={{ color: 'var(--color-text-secondary)' }}>
            <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
            <p>没有找到日志</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
                  <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>时间</th>
                  <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>级别</th>
                  <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>来源</th>
                  <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>设备</th>
                  <th className="text-left py-2 px-3 font-medium" style={{ color: 'var(--color-text-secondary)' }}>消息</th>
                </tr>
              </thead>
              <tbody>
                {filteredLogs.map((log) => (
                  <tr
                    key={log.id}
                    className="hover:bg-surface-elevated transition-colors"
                    style={{ borderBottom: '1px solid var(--color-border-subtle)' }}
                  >
                    <td className="py-2 px-3 font-mono whitespace-nowrap" style={{ color: 'var(--color-text-muted)', fontSize: '12px' }}>
                      {formatTime(log.timestamp)}
                    </td>
                    <td className="py-2 px-3">
                      <LevelBadge level={log.level} />
                    </td>
                    <td className="py-2 px-3" style={{ color: 'var(--color-text-secondary)' }}>
                      {log.source}
                    </td>
                    <td className="py-2 px-3" style={{ color: 'var(--color-text-muted)' }}>
                      {log.metadata?.device ? String(log.metadata.device) : '-'}
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
      </Card>
    </div>
  )
}