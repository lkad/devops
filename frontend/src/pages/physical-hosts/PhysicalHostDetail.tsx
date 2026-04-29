import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { ArrowLeft, Send } from 'lucide-react'
import { physicalHostsApi } from '@/api/endpoints/physicalHosts'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { useToast } from '@/components/ui/Toast'
import styles from './PhysicalHostDetail.module.css'

export function PhysicalHostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { addToast } = useToast()

  const [selectedConfig, setSelectedConfig] = useState('')

  const { data: host, isLoading } = useQuery({
    queryKey: ['physical-host', id],
    queryFn: () => physicalHostsApi.get(id!),
    enabled: !!id,
  })

  const configMutation = useMutation({
    mutationFn: (_configName: string) =>
      physicalHostsApi.update(id!, { }),
    onSuccess: () => {
      addToast({ type: 'success', message: 'Config pushed successfully' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to push config' })
    },
  })

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  if (!host) {
    return <div className={styles.container}>Host not found</div>
  }

  const configs = ['base-config', 'network-config', 'security-config', 'monitoring-config']

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/physical-hosts')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{host.name}</h1>
      </div>

      <div className={styles.metricsGrid}>
        <Card className={styles.metricCard}>
          <span className={styles.metricLabel}>CPU Usage</span>
          <span className={styles.metricValue}>{host.cpu}%</span>
          <div className={styles.metricBar}>
            <div
              className={`${styles.metricFill} ${styles.metricFillCpu}`}
              style={{ width: `${host.cpu}%` }}
            />
          </div>
        </Card>

        <Card className={styles.metricCard}>
          <span className={styles.metricLabel}>Memory Usage</span>
          <span className={styles.metricValue}>{host.memory}%</span>
          <div className={styles.metricBar}>
            <div
              className={`${styles.metricFill} ${styles.metricFillMemory}`}
              style={{ width: `${host.memory}%` }}
            />
          </div>
        </Card>

        <Card className={styles.metricCard}>
          <span className={styles.metricLabel}>Disk Usage</span>
          <span className={styles.metricValue}>{host.disk}%</span>
          <div className={styles.metricBar}>
            <div
              className={`${styles.metricFill} ${styles.metricFillDisk}`}
              style={{ width: `${host.disk}%` }}
            />
          </div>
        </Card>
      </div>

      <div className={styles.servicesSection}>
        <h2 className={styles.sectionTitle}>Services</h2>
        <div className={styles.serviceList}>
          {Array.from({ length: host.services }, (_, i) => (
            <div key={i} className={styles.serviceItem}>
              <span className={styles.serviceName}>Service {i + 1}</span>
              <span className={styles.serviceStatus}>Running</span>
            </div>
          ))}
        </div>
      </div>

      <div className={styles.configSection}>
        <h2 className={styles.sectionTitle}>Push Config</h2>
        <div className={styles.configForm}>
          <select
            value={selectedConfig}
            onChange={(e) => setSelectedConfig(e.target.value)}
            className={styles.configSelect}
          >
            <option value="">Select config...</option>
            {configs.map(config => (
              <option key={config} value={config}>{config}</option>
            ))}
          </select>
          <Button
            variant="primary"
            disabled={!selectedConfig}
            onClick={() => {
              if (selectedConfig) {
                configMutation.mutate(selectedConfig)
              }
            }}
          >
            <Send size={16} />
            Push Config
          </Button>
        </div>
      </div>
    </div>
  )
}