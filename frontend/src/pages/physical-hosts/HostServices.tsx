import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function HostServices() {
  const { id } = useParams()
  return (
    <PageContainer title="Host Services" description={`Services for host ${id}`}>
      <div>Host services coming soon</div>
    </PageContainer>
  )
}