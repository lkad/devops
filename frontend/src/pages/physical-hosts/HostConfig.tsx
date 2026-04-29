import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function HostConfig() {
  const { id } = useParams()
  return (
    <PageContainer title="Host Configuration" description={`Configuration for host ${id}`}>
      <div>Host config coming soon</div>
    </PageContainer>
  )
}