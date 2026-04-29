import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function ProjectResources() {
  const { id } = useParams()
  return (
    <PageContainer title="Project Resources" description={`Resources for project ${id}`}>
      <div>Project resources coming soon</div>
    </PageContainer>
  )
}