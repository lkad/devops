import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function ClusterPods() {
  const { cluster } = useParams()
  return (
    <PageContainer title="Cluster Pods" description={`Pods in cluster ${cluster}`}>
      <div>Cluster pods coming soon</div>
    </PageContainer>
  )
}