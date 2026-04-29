import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { kubernetesApi, type K8sNode, type K8sPod } from '@/api/endpoints/kubernetes'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import styles from './ClusterDetail.module.css'

type TabType = 'overview' | 'nodes' | 'pods' | 'namespaces'

const mockNodes: K8sNode[] = [
  { id: '1', name: 'node-01', status: 'Ready', role: 'control-plane', age: '30d', version: '1.28.0' },
  { id: '2', name: 'node-02', status: 'Ready', role: 'worker', age: '25d', version: '1.28.0' },
  { id: '3', name: 'node-03', status: 'NotReady', role: 'worker', age: '20d', version: '1.28.0' },
]

const mockPods: K8sPod[] = [
  { id: '1', name: 'nginx-abc123', namespace: 'default', status: 'Running', node: 'node-01', age: '5d' },
  { id: '2', name: 'redis-def456', namespace: 'cache', status: 'Running', node: 'node-02', age: '10d' },
  { id: '3', name: 'api-ghi789', namespace: 'api', status: 'Pending', node: 'node-02', age: '2d' },
  { id: '4', name: 'worker-jkl012', namespace: 'workers', status: 'Failed', node: 'node-03', age: '1d' },
]

export function ClusterDetail() {
  const { clusterId } = useParams<{ clusterId: string }>()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<TabType>('overview')

  const { data: cluster, isLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterId],
    queryFn: () => kubernetesApi.getCluster(clusterId!),
  })

  const { data: nodesData } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterId, 'nodes'],
    queryFn: () => kubernetesApi.getNodes(clusterId!),
    enabled: activeTab === 'nodes',
  })

  const { data: podsData } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterId, 'pods'],
    queryFn: () => kubernetesApi.getPods(clusterId!),
    enabled: activeTab === 'pods',
  })

  const { data: namespacesData } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterId, 'namespaces'],
    queryFn: () => kubernetesApi.getNamespaces(clusterId!),
    enabled: activeTab === 'namespaces',
  })

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  const clusterData = cluster || {
    id: clusterId,
    name: clusterId || 'Unknown',
    type: 'unknown',
    status: 'unknown',
    version: '1.28.0',
    environment: 'unknown',
    nodes: mockNodes.length,
    pods: mockPods.length,
    namespaces: 8,
  }

  const nodes = nodesData?.nodes ?? mockNodes
  const pods = podsData?.pods ?? mockPods
  const namespaces = namespacesData?.namespaces ?? [
    { id: '1', name: 'default', status: 'Active', labels: {} },
    { id: '2', name: 'kube-system', status: 'Active', labels: {} },
    { id: '3', name: 'monitoring', status: 'Active', labels: {} },
  ]

  const getMetricFillClass = (value: number) => {
    if (value < 50) return styles.metricFillLow
    if (value < 80) return styles.metricFillMedium
    return styles.metricFillHigh
  }

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'Ready':
      case 'Running':
        return styles.statusReady
      case 'NotReady':
      case 'Failed':
        return styles.statusFailed
      case 'Pending':
        return styles.statusPending
      default:
        return styles.statusReady
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/kubernetes')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{clusterData.name}</h1>
        <Button variant="secondary">Refresh</Button>
      </div>

      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${activeTab === 'overview' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('overview')}
        >
          Overview
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'nodes' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('nodes')}
        >
          Nodes
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'pods' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('pods')}
        >
          Pods
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'namespaces' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('namespaces')}
        >
          Namespaces
        </button>
      </div>

      {activeTab === 'overview' && (
        <div>
          <div className={styles.overviewGrid}>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{clusterData.nodes}</div>
              <div className={styles.overviewLabel}>Nodes</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{clusterData.pods}</div>
              <div className={styles.overviewLabel}>Pods</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{clusterData.namespaces}</div>
              <div className={styles.overviewLabel}>Namespaces</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{clusterData.version}</div>
              <div className={styles.overviewLabel}>Version</div>
            </Card>
          </div>
        </div>
      )}

      {activeTab === 'nodes' && (
        <div className={styles.tableContainer}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--color-surface-elevated)' }}>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Name</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Status</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Role</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>CPU</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Memory</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map((node) => (
                <tr key={node.id} className={styles.nodeRow}>
                  <td className={`${styles.nodeCell} ${styles.monoCell}`}>{node.name}</td>
                  <td className={styles.nodeCell}>
                    <span className={`${styles.statusBadge} ${getStatusBadgeClass(node.status)}`}>
                      {node.status}
                    </span>
                  </td>
                  <td className={styles.nodeCell}>{node.role}</td>
                  <td className={styles.nodeCell}>
                    <div className={styles.metricBar}>
                      <div className={`${styles.metricFill} ${getMetricFillClass(35)}`} style={{ width: '35%' }} />
                    </div>
                  </td>
                  <td className={styles.nodeCell}>
                    <div className={styles.metricBar}>
                      <div className={`${styles.metricFill} ${getMetricFillClass(55)}`} style={{ width: '55%' }} />
                    </div>
                  </td>
                  <td className={styles.nodeCell}>{node.age}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === 'pods' && (
        <div className={styles.tableContainer}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--color-surface-elevated)' }}>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Name</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Status</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Namespace</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Node</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>CPU</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Memory</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
              </tr>
            </thead>
            <tbody>
              {pods.map((pod) => (
                <tr key={pod.id} className={styles.podRow}>
                  <td className={`${styles.podCell} ${styles.monoCell}`}>{pod.name}</td>
                  <td className={styles.podCell}>
                    <span className={`${styles.statusBadge} ${getStatusBadgeClass(pod.status)}`}>
                      {pod.status}
                    </span>
                  </td>
                  <td className={styles.podCell}>{pod.namespace}</td>
                  <td className={`${styles.podCell} ${styles.monoCell}`}>{pod.node}</td>
                  <td className={styles.podCell}>
                    <div className={styles.metricBar}>
                      <div className={`${styles.metricFill} ${getMetricFillClass(20)}`} style={{ width: '20%' }} />
                    </div>
                  </td>
                  <td className={styles.podCell}>
                    <div className={styles.metricBar}>
                      <div className={`${styles.metricFill} ${getMetricFillClass(40)}`} style={{ width: '40%' }} />
                    </div>
                  </td>
                  <td className={styles.podCell}>{pod.age}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {activeTab === 'namespaces' && (
        <div className={styles.tableContainer}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--color-surface-elevated)' }}>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Name</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {namespaces.map((ns) => (
                <tr key={ns.id} className={styles.podRow}>
                  <td className={`${styles.podCell} ${styles.monoCell}`}>{ns.name}</td>
                  <td className={styles.podCell}>
                    <span className={`${styles.statusBadge} ${styles.statusReady}`}>
                      {ns.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}