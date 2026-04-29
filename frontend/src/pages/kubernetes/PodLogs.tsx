import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function PodLogs() {
  const { cluster, pod } = useParams()
  return (
    <PageContainer title="Pod Logs" description={`Logs for pod ${pod} in ${cluster}`}>
      <div>Pod logs coming soon</div>
    </PageContainer>
  )
}