import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { kubernetesApi, type K8sCluster } from '@/api/endpoints/kubernetes'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './ClusterList.module.css'

const envBadgeClass = (env: string): string => {
  switch (env.toLowerCase()) {
    case 'dev': return styles.envDev
    case 'test': return styles.envTest
    case 'uat': return styles.envUat
    case 'prod': return styles.envProd
    default: return styles.envDev
  }
}

const healthClass = (status: string): string => {
  switch (status.toLowerCase()) {
    case 'healthy':
    case 'online':
    case 'running':
      return styles.healthHealthy
    case 'degraded':
    case 'pending':
      return styles.healthDegraded
    case 'unhealthy':
    case 'offline':
      return styles.healthUnhealthy
    default:
      return styles.healthDegraded
  }
}

export function ClusterList() {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  const [envFilter, setEnvFilter] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['kubernetes', 'clusters'],
    queryFn: async () => {
      const response = await kubernetesApi.listClusters()
      return response.data ?? []
    },
  })

  const clusters = data ?? []

  const filteredClusters = useMemo(() => {
    return clusters.filter(cluster => {
      const matchesSearch = cluster.name.toLowerCase().includes(searchQuery.toLowerCase())
      const matchesEnv = !envFilter || (cluster.environment?.toLowerCase() === envFilter.toLowerCase())
      return matchesSearch && matchesEnv
    })
  }, [clusters, searchQuery, envFilter])

  const handleClusterClick = (cluster: K8sCluster) => {
    navigate(`/k8s/${cluster.name}`)
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Kubernetes Clusters</h1>
      </div>

      <div className={styles.filters}>
        <input
          type="text"
          placeholder="Search by name..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className={styles.searchInput}
        />
        <select
          value={envFilter}
          onChange={(e) => setEnvFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Environments</option>
          <option value="dev">Dev</option>
          <option value="test">Test</option>
          <option value="uat">UAT</option>
          <option value="prod">Prod</option>
        </select>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading clusters...</div>
      ) : filteredClusters.length === 0 ? (
        <EmptyState
          title="No clusters found"
          description={searchQuery || envFilter ? "Try adjusting your search or filter criteria" : "Get started by adding your first Kubernetes cluster"}
        />
      ) : (
        <div className={styles.clustersGrid}>
          {filteredClusters.map(cluster => (
            <Card
              key={cluster.name}
              className={styles.clusterCard}
              onClick={() => handleClusterClick(cluster)}
            >
              <div className={styles.clusterHeader}>
                <div>
                  <h3 className={styles.clusterName}>{cluster.name}</h3>
                  <span className={styles.clusterVersion}>v{cluster.version || '1.0'}</span>
                </div>
                <span className={`${styles.environmentBadge} ${envBadgeClass(cluster.environment || 'dev')}`}>
                  {(cluster.environment || 'dev').toUpperCase()}
                </span>
              </div>

              <div className={styles.healthIndicator}>
                <span className={`${styles.healthDot} ${healthClass(cluster.status)}`} />
                <span>{cluster.status}</span>
              </div>

              <div className={styles.clusterStats}>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.nodes ?? '-'}</div>
                  <div className={styles.statLabel}>Nodes</div>
                </div>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.pods ?? '-'}</div>
                  <div className={styles.statLabel}>Pods</div>
                </div>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.namespaces ?? '-'}</div>
                  <div className={styles.statLabel}>Namespaces</div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}