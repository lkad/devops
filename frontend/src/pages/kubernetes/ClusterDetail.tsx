import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { kubernetesApi, type K8sCluster, type K8sNode, type K8sPod, type K8sNamespace } from '@/api/endpoints/kubernetes'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import styles from './ClusterDetail.module.css'

type TabType = 'overview' | 'nodes' | 'pods' | 'namespaces'

export function ClusterDetail() {
  const { cluster: clusterName } = useParams<{ cluster: string }>()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<TabType>('overview')

  // Fetch all clusters and find the current one
  const { data: clustersResponse } = useQuery({
    queryKey: ['kubernetes', 'clusters'],
    queryFn: () => kubernetesApi.listClusters(),
    enabled: !!clusterName,
  })

  const clusterData = clustersResponse?.data?.find((c: K8sCluster) => c.name === clusterName)

  const { data: nodesResponse } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterName, 'nodes'],
    queryFn: () => kubernetesApi.getNodes(clusterName!),
    enabled: activeTab === 'nodes' && !!clusterName,
  })

  const { data: podsResponse } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterName, 'pods'],
    queryFn: () => kubernetesApi.getPods(clusterName!),
    enabled: activeTab === 'pods' && !!clusterName,
  })

  const { data: namespacesResponse } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterName, 'namespaces'],
    queryFn: () => kubernetesApi.getNamespaces(clusterName!),
    enabled: activeTab === 'namespaces' && !!clusterName,
  })

  const nodes: K8sNode[] = nodesResponse?.data ?? []
  const pods: K8sPod[] = podsResponse?.data ?? []
  const namespaces: K8sNamespace[] = namespacesResponse?.data ?? []

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'Ready':
      case 'Running':
      case 'Active':
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

  if (!clusterName) {
    return <div className={styles.container}>Cluster not found</div>
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/k8s')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{clusterName}</h1>
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
          Nodes ({nodes.length})
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'pods' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('pods')}
        >
          Pods ({pods.length})
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'namespaces' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('namespaces')}
        >
          Namespaces ({namespaces.length})
        </button>
      </div>

      {activeTab === 'overview' && (
        <div>
          <div className={styles.overviewGrid}>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{(clusterData?.nodes) ?? nodes.length ?? '-'}</div>
              <div className={styles.overviewLabel}>Nodes</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{(clusterData?.pods) ?? pods.length ?? '-'}</div>
              <div className={styles.overviewLabel}>Pods</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{(clusterData?.namespaces) ?? namespaces.length ?? '-'}</div>
              <div className={styles.overviewLabel}>Namespaces</div>
            </Card>
            <Card className={styles.overviewCard}>
              <div className={styles.overviewValue}>{clusterData?.version ?? clusterData?.type ?? 'k3d'}</div>
              <div className={styles.overviewLabel}>Type</div>
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
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map((node) => (
                <tr key={node.name} className={styles.nodeRow}>
                  <td className={`${styles.nodeCell} ${styles.monoCell}`}>{node.name}</td>
                  <td className={styles.nodeCell}>
                    <span className={`${styles.statusBadge} ${getStatusBadgeClass(node.status)}`}>
                      {node.status}
                    </span>
                  </td>
                  <td className={styles.nodeCell}>{node.role}</td>
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
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
              </tr>
            </thead>
            <tbody>
              {pods.map((pod) => (
                <tr key={pod.name} className={styles.podRow}>
                  <td className={`${styles.podCell} ${styles.monoCell}`}>{pod.name}</td>
                  <td className={styles.podCell}>
                    <span className={`${styles.statusBadge} ${getStatusBadgeClass(pod.status)}`}>
                      {pod.status}
                    </span>
                  </td>
                  <td className={styles.podCell}>{pod.namespace}</td>
                  <td className={`${styles.podCell} ${styles.monoCell}`}>{pod.node}</td>
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
                <tr key={ns.name} className={styles.podRow}>
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