import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function ClusterNodes() {
  const { cluster } = useParams()
  return (
    <PageContainer title="Cluster Nodes" description={`Nodes in cluster ${cluster}`}>
      <div>Cluster nodes coming soon</div>
    </PageContainer>
  )
}