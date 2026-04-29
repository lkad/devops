import { useParams } from 'react-router-dom'
import { PageContainer } from '@/components/layout'

export function PodExec() {
  const { pod } = useParams()
  return (
    <PageContainer title="Pod Exec" description={`Execute commands in pod ${pod}`}>
      <div>Pod exec coming soon</div>
    </PageContainer>
  )
}