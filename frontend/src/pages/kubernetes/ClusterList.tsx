import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { kubernetesApi, type K8sCluster } from '@/api/endpoints/kubernetes'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './ClusterList.module.css'

const mockClusters: K8sCluster[] = [
  { id: '1', name: 'dev-cluster-01', type: 'dev', status: 'online', version: '1.28.0', environment: 'dev', nodes: 3, pods: 45, namespaces: 8 },
  { id: '2', name: 'test-cluster-01', type: 'test', status: 'online', version: '1.28.0', environment: 'test', nodes: 5, pods: 120, namespaces: 15 },
  { id: '3', name: 'uat-cluster-01', type: 'uat', status: 'pending', version: '1.27.0', environment: 'uat', nodes: 5, pods: 98, namespaces: 12 },
  { id: '4', name: 'prod-cluster-01', type: 'prod', status: 'online', version: '1.28.0', environment: 'prod', nodes: 7, pods: 234, namespaces: 25 },
  { id: '5', name: 'prod-cluster-02', type: 'prod', status: 'offline', version: '1.28.0', environment: 'prod', nodes: 7, pods: 198, namespaces: 22 },
  { id: '6', name: 'dev-cluster-02', type: 'dev', status: 'active', version: '1.29.0', environment: 'dev', nodes: 2, pods: 28, namespaces: 5 },
]

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
      try {
        const response = await kubernetesApi.listClusters()
        return response.clusters
      } catch {
        return mockClusters
      }
    },
  })

  const clusters = data ?? []

  const filteredClusters = useMemo(() => {
    return clusters.filter(cluster => {
      const matchesSearch = cluster.name.toLowerCase().includes(searchQuery.toLowerCase())
      const matchesEnv = !envFilter || cluster.environment.toLowerCase() === envFilter.toLowerCase()
      return matchesSearch && matchesEnv
    })
  }, [clusters, searchQuery, envFilter])

  const handleClusterClick = (cluster: K8sCluster) => {
    navigate(`/kubernetes/${cluster.id}`)
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
              key={cluster.id}
              className={styles.clusterCard}
              onClick={() => handleClusterClick(cluster)}
            >
              <div className={styles.clusterHeader}>
                <div>
                  <h3 className={styles.clusterName}>{cluster.name}</h3>
                  <span className={styles.clusterVersion}>v{cluster.version}</span>
                </div>
                <span className={`${styles.environmentBadge} ${envBadgeClass(cluster.environment)}`}>
                  {cluster.environment.toUpperCase()}
                </span>
              </div>

              <div className={styles.healthIndicator}>
                <span className={`${styles.healthDot} ${healthClass(cluster.status)}`} />
                <span>{cluster.status}</span>
              </div>

              <div className={styles.clusterStats}>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.nodes}</div>
                  <div className={styles.statLabel}>Nodes</div>
                </div>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.pods}</div>
                  <div className={styles.statLabel}>Pods</div>
                </div>
                <div className={styles.stat}>
                  <div className={styles.statValue}>{cluster.namespaces}</div>
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