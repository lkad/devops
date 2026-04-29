import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function ClusterNamespaces() {
  const { cluster } = useParams()
  return (
    <PageContainer title="Cluster Namespaces" description={`Namespaces in cluster ${cluster}`}>
      <div>Cluster namespaces coming soon</div>
    </PageContainer>
  )
}