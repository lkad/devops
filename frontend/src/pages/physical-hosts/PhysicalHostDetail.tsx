import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { ArrowLeft, Send, Box, ChevronRight } from 'lucide-react'
import { physicalHostsApi, type PhysicalHost } from '@/api/endpoints/physicalHosts'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { useToast } from '@/components/ui/Toast'
import styles from './PhysicalHostDetail.module.css'

interface K8sPod {
  name: string
  namespace: string
  status: string
  cluster: string
  node: string
  age: string
}

export function PhysicalHostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { addToast } = useToast()
  const [selectedConfig, setSelectedConfig] = useState('')
  const [activeTab, setActiveTab] = useState<'details' | 'services' | 'config' | 'k8s-pods'>('details')

  const { data: host, isLoading } = useQuery({
    queryKey: ['physical-host', id],
    queryFn: () => physicalHostsApi.getHost(id!),
    enabled: !!id,
  })

  // Fetch K8s pods running on this host
  const { data: k8sPodsResponse } = useQuery({
    queryKey: ['physical-host', id, 'k8s-pods'],
    queryFn: async () => {
      // In production: return apiClient.get(`/api/physical-hosts/${id}/k8s-pods`)
      // Mock data for demonstration
      return {
        data: [
          { name: 'nginx-7d8f9f6e4-x2k9p', namespace: 'default', status: 'Running', cluster: 'prod-cluster', node: 'worker-1', age: '15d' },
          { name: 'redis-6b7d8f9e4-q1r3t', namespace: 'cache', status: 'Running', cluster: 'prod-cluster', node: 'worker-1', age: '30d' },
          { name: 'api-gateway-5c6d8f9e4-s4u6v', namespace: 'api', status: 'Running', cluster: 'prod-cluster', node: 'worker-2', age: '7d' },
        ] as K8sPod[]
      }
    },
    enabled: activeTab === 'k8s-pods' && !!id,
  })

  const configMutation = useMutation({
    mutationFn: (_configName: string) =>
      physicalHostsApi.pushConfig(id!, _configName),
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

  const typedHost = host as PhysicalHost
  const configs = ['base-config', 'network-config', 'security-config', 'monitoring-config']
  const k8sPods: K8sPod[] = k8sPodsResponse?.data ?? []

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/physical-hosts')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{typedHost.hostname}</h1>
      </div>

      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${activeTab === 'details' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('details')}
        >
          Details
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'services' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('services')}
        >
          Services
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'config' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('config')}
        >
          Config
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'k8s-pods' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('k8s-pods')}
        >
          <Box size={14} style={{ marginRight: '4px' }} />
          K8s Pods
        </button>
      </div>

      {activeTab === 'details' && (
        <div className={styles.metricsGrid}>
          <Card className={styles.metricCard}>
            <span className={styles.metricLabel}>CPU Usage</span>
            <span className={styles.metricValue}>{typedHost.metrics?.cpu?.usage ?? 0}%</span>
            <div className={styles.metricBar}>
              <div
                className={`${styles.metricFill} ${styles.metricFillCpu}`}
                style={{ width: `${typedHost.metrics?.cpu?.usage ?? 0}%` }}
              />
            </div>
          </Card>

          <Card className={styles.metricCard}>
            <span className={styles.metricLabel}>Memory Usage</span>
            <span className={styles.metricValue}>{typedHost.metrics?.memory?.usagePercent ?? 0}%</span>
            <div className={styles.metricBar}>
              <div
                className={`${styles.metricFill} ${styles.metricFillMemory}`}
                style={{ width: `${typedHost.metrics?.memory?.usagePercent ?? 0}%` }}
              />
            </div>
          </Card>

          <Card className={styles.metricCard}>
            <span className={styles.metricLabel}>Status</span>
            <span className={styles.metricValue}>{typedHost.state ?? 'unknown'}</span>
          </Card>
        </div>
      )}

      {activeTab === 'services' && (
        <div className={styles.servicesSection}>
          <h2 className={styles.sectionTitle}>Host Information</h2>
          <div className={styles.serviceList}>
            <div className={styles.serviceItem}>
              <span className={styles.serviceName}>IP Address</span>
              <span className={styles.serviceStatus}>{typedHost.ip}</span>
            </div>
            <div className={styles.serviceItem}>
              <span className={styles.serviceName}>Port</span>
              <span className={styles.serviceStatus}>{typedHost.port}</span>
            </div>
            <div className={styles.serviceItem}>
              <span className={styles.serviceName}>Monitoring</span>
              <span className={styles.serviceStatus}>{typedHost.monitoringStatus}</span>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'config' && (
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
      )}

      {activeTab === 'k8s-pods' && (
        <div className={styles.k8sSection}>
          <h2 className={styles.sectionTitle}>Kubernetes Pods</h2>
          <p className={styles.sectionDescription}>
            Pods running on this physical host across all clusters
          </p>
          {k8sPods.length === 0 ? (
            <p className={styles.emptyText}>No Kubernetes pods found on this host</p>
          ) : (
            <Card>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ background: 'var(--color-surface-elevated)' }}>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Pod Name</th>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Namespace</th>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Cluster</th>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Status</th>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
                    <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}></th>
                  </tr>
                </thead>
                <tbody>
                  {k8sPods.map((pod) => (
                    <tr key={pod.name} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                      <td style={{ padding: '12px 16px', fontFamily: 'var(--font-mono)', fontSize: '13px', color: 'var(--color-text-primary)' }}>{pod.name}</td>
                      <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>{pod.namespace}</td>
                      <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>{pod.cluster}</td>
                      <td style={{ padding: '12px 16px' }}>
                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', color: 'var(--color-success)' }}>
                          <span style={{ width: '6px', height: '6px', borderRadius: '50%', background: 'var(--color-success)' }} />
                          {pod.status}
                        </span>
                      </td>
                      <td style={{ padding: '12px 16px', color: 'var(--color-text-muted)' }}>{pod.age}</td>
                      <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                        <button
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '4px',
                            background: 'none',
                            border: 'none',
                            color: 'var(--color-primary)',
                            cursor: 'pointer',
                            fontSize: '13px',
                            padding: '4px 8px',
                          }}
                          onClick={() => navigate(`/k8s/${pod.cluster}/pods`)}
                        >
                          View <ChevronRight size={14} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          )}
        </div>
      )}
    </div>
  )
}