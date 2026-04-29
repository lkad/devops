import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function HostDetail() {
  const { id } = useParams()
  return (
    <PageContainer title="Host Details" description={`Host ID: ${id}`}>
      <div>Host details coming soon</div>
    </PageContainer>
  )
}