import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function ProjectPermissions() {
  const { id } = useParams()
  return (
    <PageContainer title="Project Permissions" description={`Permissions for project ${id}`}>
      <div>Project permissions coming soon</div>
    </PageContainer>
  )
}