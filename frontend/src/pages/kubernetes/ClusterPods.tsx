import { useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Filter, X, FileText } from 'lucide-react'
import { kubernetesApi, type K8sPod } from '@/api/endpoints/kubernetes'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import styles from './ClusterPods.module.css'

export function ClusterPods() {
  const { cluster: clusterName } = useParams<{ cluster: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [nodeFilter, setNodeFilter] = useState<string>(searchParams.get('node') || '')
  const [namespaceFilter, setNamespaceFilter] = useState<string>(searchParams.get('namespace') || '')
  const [showFilters, setShowFilters] = useState(false)

  const { data: podsResponse, isLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', clusterName, 'pods'],
    queryFn: () => kubernetesApi.getPods(clusterName!),
    enabled: !!clusterName,
  })

  const pods: K8sPod[] = podsResponse?.data ?? []

  // Get unique nodes and namespaces for filter dropdowns
  const uniqueNodes = [...new Set(pods.map(p => p.node))].sort()
  const uniqueNamespaces = [...new Set(pods.map(p => p.namespace))].sort()

  // Filter pods based on selected filters
  const filteredPods = pods.filter(pod => {
    if (nodeFilter && pod.node !== nodeFilter) return false
    if (namespaceFilter && pod.namespace !== namespaceFilter) return false
    return true
  })

  const handleNodeClick = (nodeName: string) => {
    // Navigate to nodes tab with this node highlighted
    navigate(`/k8s/${clusterName}/nodes?highlight=${encodeURIComponent(nodeName)}`)
  }

  const clearFilters = () => {
    setNodeFilter('')
    setNamespaceFilter('')
    setSearchParams({})
  }

  const hasActiveFilters = nodeFilter || namespaceFilter

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'Running':
        return styles.statusRunning
      case 'Pending':
        return styles.statusPending
      case 'Failed':
        return styles.statusFailed
      default:
        return styles.statusRunning
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
        <h1 className={styles.title}>Pods - {clusterName}</h1>
        <Button
          variant="secondary"
          size="sm"
          leftIcon={<Filter size={14} />}
          onClick={() => setShowFilters(!showFilters)}
        >
          Filters
        </Button>
      </div>

      {showFilters && (
        <Card className={styles.filtersCard}>
          <div className={styles.filtersRow}>
            <div className={styles.filterGroup}>
              <label className={styles.filterLabel}>Node</label>
              <select
                value={nodeFilter}
                onChange={(e) => setNodeFilter(e.target.value)}
                className={styles.filterSelect}
              >
                <option value="">All Nodes</option>
                {uniqueNodes.map(node => (
                  <option key={node} value={node}>{node}</option>
                ))}
              </select>
            </div>
            <div className={styles.filterGroup}>
              <label className={styles.filterLabel}>Namespace</label>
              <select
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
                className={styles.filterSelect}
              >
                <option value="">All Namespaces</option>
                {uniqueNamespaces.map(ns => (
                  <option key={ns} value={ns}>{ns}</option>
                ))}
              </select>
            </div>
            {hasActiveFilters && (
              <Button variant="ghost" size="sm" onClick={clearFilters}>
                <X size={14} /> Clear
              </Button>
            )}
          </div>
        </Card>
      )}

      {isLoading ? (
        <div className={styles.loading}>Loading pods...</div>
      ) : filteredPods.length === 0 ? (
        <div className={styles.empty}>
          {hasActiveFilters ? 'No pods match the selected filters' : 'No pods found in this cluster'}
        </div>
      ) : (
        <>
          <div className={styles.podCount}>
            Showing {filteredPods.length} of {pods.length} pods
          </div>
          <Card>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: 'var(--color-surface-elevated)' }}>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Name</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Status</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Namespace</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Node</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Age</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredPods.map((pod) => (
                  <tr key={pod.name} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                    <td className={`${styles.podCell} ${styles.monoCell}`}>{pod.name}</td>
                    <td className={styles.podCell}>
                      <span className={`${styles.statusBadge} ${getStatusBadgeClass(pod.status)}`}>
                        {pod.status}
                      </span>
                    </td>
                    <td className={styles.podCell}>{pod.namespace}</td>
                    <td className={styles.podCell}>
                      <button
                        className={styles.nodeLink}
                        onClick={() => handleNodeClick(pod.node)}
                        title={`View node ${pod.node}`}
                      >
                        {pod.node}
                      </button>
                    </td>
                    <td className={styles.podCell}>{pod.age}</td>
                    <td className={styles.podCell}>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/k8s/${clusterName}/namespaces/${pod.namespace}/pods/${pod.name}/logs`)}
                        title="View logs"
                      >
                        <FileText size={14} />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        </>
      )}
    </div>
  )
}