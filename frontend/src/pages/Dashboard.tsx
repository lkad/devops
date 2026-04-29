import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Server,
  AlertCircle,
  GitBranch,
  Monitor,
  Database,
  Bell,
} from 'lucide-react'
import { devicesApi } from '@/api/endpoints/devices'
import { pipelinesApi } from '@/api/endpoints/pipelines'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { StatsGrid, StatsCard } from '@/components/ui/StatsCard'
import styles from './Dashboard.module.css'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
    case 'running':
    case 'online':
      return 'success'
    case 'pending':
    case 'monitoring_issue':
      return 'warning'
    case 'inactive':
    case 'offline':
    case 'failed':
      return 'error'
    default:
      return 'default'
  }
}

const formatTime = (timestamp: string | null | undefined): string => {
  if (!timestamp) return 'Never'
  const date = new Date(timestamp)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

export function Dashboard() {
  const { data: devicesData, isLoading: devicesLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: () => devicesApi.list(),
  })

  const { data: pipelinesData, isLoading: pipelinesLoading } = useQuery({
    queryKey: ['pipelines'],
    queryFn: () => pipelinesApi.list(),
  })

  const devices = devicesData?.devices ?? []
  const pipelines = pipelinesData?.pipelines ?? []

  const activeDevices = devices.filter(d => d.status.toLowerCase() === 'active').length
  const totalDevices = devices.length

  const activePipelines = pipelines.filter(p => p.status === 'running').length
  const totalPipelines = pipelines.length

  const recentDevices = [...devices]
    .sort((a, b) => {
      const dateA = a.lastSeen || a.registeredAt || ''
      const dateB = b.lastSeen || b.registeredAt || ''
      return dateB.localeCompare(dateA)
    })
    .slice(0, 5)

  const quickActions = [
    { to: '/devices/new', icon: Server, text: 'Register Device' },
    { to: '/pipelines/new', icon: GitBranch, text: 'Create Pipeline' },
    { to: '/logs', icon: Monitor, text: 'View Logs' },
    { to: '/alerts', icon: Bell, text: 'View Alerts' },
  ]

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Dashboard</h1>
      </div>

      <StatsGrid className={styles.statsGrid}>
        <StatsCard
          label="Devices"
          value={devicesLoading ? '-' : totalDevices}
          icon={<Server size={20} />}
          iconColor="var(--color-primary)"
          iconBg="var(--color-primary-muted)"
          trend={activeDevices > 0 ? { value: Math.round((activeDevices / totalDevices) * 100), label: 'active' } : undefined}
        />
        <StatsCard
          label="Pipelines"
          value={pipelinesLoading ? '-' : totalPipelines}
          icon={<GitBranch size={20} />}
          iconColor="var(--color-success)"
          iconBg="var(--color-success-muted)"
          trend={activePipelines > 0 ? { value: Math.round((activePipelines / totalPipelines) * 100), label: 'running' } : undefined}
        />
        <StatsCard
          label="Alerts"
          value={0}
          icon={<AlertCircle size={20} />}
          iconColor="var(--color-warning)"
          iconBg="var(--color-warning-muted)"
        />
        <StatsCard
          label="K8s Clusters"
          value={0}
          icon={<Database size={20} />}
          iconColor="var(--color-info)"
          iconBg="var(--color-info-muted)"
        />
      </StatsGrid>

      <div className={styles.contentGrid}>
        <Card className={styles.section}>
          <h2 className={styles.sectionTitle}>Recent Activity</h2>
          {devicesLoading ? (
            <div className={styles.emptyState}>Loading...</div>
          ) : recentDevices.length === 0 ? (
            <div className={styles.emptyState}>No recent activity</div>
          ) : (
            <div className={styles.activityList}>
              {recentDevices.map(device => (
                <div key={device.id} className={styles.activityItem}>
                  <div className={styles.activityIcon}>
                    <Server size={16} />
                  </div>
                  <div className={styles.activityContent}>
                    <p className={styles.activityName}>{device.name}</p>
                    <p className={styles.activityMeta}>{device.type} • {device.environment || 'unknown'}</p>
                  </div>
                  <Badge variant={statusVariant(device.status)}>
                    {device.status}
                  </Badge>
                  <span className={styles.activityTime}>
                    {formatTime(device.lastSeen || device.registeredAt)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card className={styles.section}>
          <h2 className={styles.sectionTitle}>Quick Actions</h2>
          <div className={styles.quickActions}>
            {quickActions.map((action, index) => (
              <Link key={index} to={action.to} className={styles.quickActionButton}>
                <span className={styles.quickActionIcon}>
                  <action.icon size={18} />
                </span>
                <span className={styles.quickActionText}>{action.text}</span>
              </Link>
            ))}
          </div>
        </Card>
      </div>
    </div>
  )
}