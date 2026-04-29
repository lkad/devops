import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { kubernetesApi, type K8sCluster, type CreateClusterRequest } from '@/api/endpoints/kubernetes'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { useToast } from '@/components/ui/Toast'
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
  const queryClient = useQueryClient()
  const { addToast } = useToast()
  const [searchQuery, setSearchQuery] = useState('')
  const [envFilter, setEnvFilter] = useState('')
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [formData, setFormData] = useState<CreateClusterRequest>({
    name: '',
    type: 'dev',
    version: '1.28',
  })

  const { data, isLoading } = useQuery({
    queryKey: ['kubernetes', 'clusters'],
    queryFn: async () => {
      const response = await kubernetesApi.listClusters()
      return response.data ?? []
    },
  })

  const clusters = data ?? []

  const createMutation = useMutation({
    mutationFn: (data: CreateClusterRequest) => kubernetesApi.createCluster(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['kubernetes', 'clusters'] })
      addToast({ type: 'success', message: 'Cluster created successfully' })
      closeForm()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to create cluster' })
    },
  })

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

  const openForm = () => {
    setFormData({ name: '', type: 'dev', version: '1.28' })
    setIsFormOpen(true)
  }

  const closeForm = () => {
    setIsFormOpen(false)
  }

  const handleSubmit = () => {
    if (!formData.name.trim()) {
      addToast({ type: 'error', message: 'Cluster name is required' })
      return
    }
    createMutation.mutate(formData)
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Kubernetes Clusters</h1>
        <Button variant="primary" onClick={openForm}>
          <Plus size={18} />
          Add Cluster
        </Button>
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

      <Modal
        isOpen={isFormOpen}
        onClose={closeForm}
        title="Create Cluster"
        footer={
          <>
            <Button variant="secondary" onClick={closeForm} disabled={createMutation.isPending}>
              Cancel
            </Button>
            <Button variant="primary" onClick={handleSubmit} loading={createMutation.isPending}>
              Create
            </Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          <Input
            label="Cluster Name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            placeholder="my-cluster"
          />
          <Select
            label="Environment"
            value={formData.type}
            onChange={(e) => setFormData({ ...formData, type: e.target.value })}
            options={[
              { value: 'dev', label: 'Development' },
              { value: 'test', label: 'Testing' },
              { value: 'uat', label: 'UAT' },
              { value: 'prod', label: 'Production' },
            ]}
          />
          <Input
            label="Kubernetes Version"
            value={formData.version || ''}
            onChange={(e) => setFormData({ ...formData, version: e.target.value })}
            placeholder="1.28"
          />
        </div>
      </Modal>
    </div>
  )
}