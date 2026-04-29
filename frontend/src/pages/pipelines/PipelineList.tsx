import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Play } from 'lucide-react'
import { pipelinesApi } from '@/api/endpoints/pipelines'
import { Button } from '@/components/ui/Button'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import { useToast } from '@/components/ui/Toast'
import { PipelineForm } from './PipelineForm'
import type { Pipeline } from '@/api/endpoints/pipelines'
import styles from './PipelineList.module.css'

interface PipelineRow {
  id: string
  name: string
  stages: string[]
  createdAt: string
}

const formatDate = (date?: string): string => {
  if (!date) return 'N/A'
  return new Date(date).toLocaleString()
}

export function PipelineList() {
  const navigate = useNavigate()
  const { addToast } = useToast()
  const queryClient = useQueryClient()
  const [isFormOpen, setIsFormOpen] = useState(false)

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

  const handleFormSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ['pipelines'] })
  }

  const pipelines: Pipeline[] = data?.pipelines ?? []

  const columns = useMemo(() => [
    {
      id: 'name',
      header: 'Name',
      accessor: (row: PipelineRow) => row.name,
    },
    {
      id: 'stages',
      header: 'Stages',
      accessor: (row: PipelineRow) => row.stages.join(' → ') || 'None',
    },
    {
      id: 'createdAt',
      header: 'Created',
      accessor: (row: PipelineRow) => formatDate(row.createdAt),
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
        <Button variant="primary" onClick={() => setIsFormOpen(true)}>
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
            onClick: () => setIsFormOpen(true)
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

      <PipelineForm
        isOpen={isFormOpen}
        onClose={() => setIsFormOpen(false)}
        onSuccess={handleFormSuccess}
      />
    </div>
  )
}