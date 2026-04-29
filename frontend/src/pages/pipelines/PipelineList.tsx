import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Plus, Play } from 'lucide-react'
import { pipelinesApi } from '@/api/endpoints/pipelines'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import { useToast } from '@/components/ui/Toast'
import styles from './PipelineList.module.css'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'running':
      return 'info'
    case 'success':
      return 'success'
    case 'failed':
      return 'error'
    case 'pending':
      return 'warning'
    default:
      return 'default'
  }
}

const formatLastRun = (lastRun: string | null): string => {
  if (!lastRun) return 'Never'
  const date = new Date(lastRun)
  return date.toLocaleString()
}

interface PipelineRow {
  id: string
  name: string
  status: string
  lastRun: string | null
  stages: string[]
  createdAt: string
}

export function PipelineList() {
  const navigate = useNavigate()
  const { addToast } = useToast()

  const { data, isLoading } = useQuery({
    queryKey: ['pipelines'],
    queryFn: () => pipelinesApi.list(),
  })

  const executeMutation = useMutation({
    mutationFn: (id: string) => pipelinesApi.execute(id),
    onSuccess: (run) => {
      addToast({ type: 'success', message: 'Pipeline started' })
      navigate(`/pipelines/${run.pipelineId}/run/${run.id}`)
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to execute pipeline' })
    },
  })

  const pipelines: PipelineRow[] = data?.pipelines ?? []

  const columns = useMemo(() => [
    {
      id: 'name',
      header: 'Name',
      accessor: (row: PipelineRow) => row.name,
    },
    {
      id: 'status',
      header: 'Status',
      accessor: (row: PipelineRow) => (
        <Badge variant={statusVariant(row.status)}>
          {row.status}
        </Badge>
      ),
    },
    {
      id: 'lastRun',
      header: 'Last Run',
      accessor: (row: PipelineRow) => formatLastRun(row.lastRun),
    },
    {
      id: 'stages',
      header: 'Stages',
      accessor: (row: PipelineRow) => row.stages.join(' → ') || 'None',
    },
    {
      id: 'actions',
      header: 'Actions',
      accessor: (row: PipelineRow) => (
        <div className={styles.actionCell}>
          <Button
            variant="secondary"
            size="sm"
            onClick={(e) => {
              e.stopPropagation()
              executeMutation.mutate(row.id)
            }}
            disabled={row.status === 'running'}
          >
            <Play size={14} />
            Run
          </Button>
        </div>
      ),
    },
  ], [])

  const handleRowClick = (row: PipelineRow) => {
    navigate(`/pipelines/${row.id}`)
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Pipelines</h1>
        <Button variant="primary" onClick={() => navigate('/pipelines/new')}>
          <Plus size={18} />
          Create Pipeline
        </Button>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading pipelines...</div>
      ) : pipelines.length === 0 ? (
        <EmptyState
          title="No pipelines found"
          description="Get started by creating your first pipeline"
          action={{
            label: "Create Pipeline",
            onClick: () => navigate('/pipelines/new')
          }}
        />
      ) : (
        <div className={styles.tableContainer}>
          <DataTable
            data={pipelines}
            columns={columns}
            onRowClick={handleRowClick}
            pageSize={10}
          />
        </div>
      )}
    </div>
  )
}