import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { ArrowLeft, Play, Check, X, Loader } from 'lucide-react'
import { pipelinesApi } from '@/api/endpoints/pipelines'
import { Button } from '@/components/ui/Button'
import { useToast } from '@/components/ui/Toast'
import styles from './PipelineDetail.module.css'

const formatDate = (date?: string): string => {
  if (!date) return 'N/A'
  return new Date(date).toLocaleString()
}

interface PipelineRun {
  id: string
  pipelineId: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  startedAt: string
  finishedAt?: string
}

export function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { addToast } = useToast()

  const [stages, setStages] = useState<string[]>([])

  const { data: pipeline, isLoading } = useQuery({
    queryKey: ['pipeline', id],
    queryFn: () => pipelinesApi.get(id!),
    enabled: !!id,
  })

  const { data: runsData } = useQuery({
    queryKey: ['pipeline-runs', id],
    queryFn: () => pipelinesApi.getRuns(id!),
    enabled: !!id,
  })

  const executeMutation = useMutation({
    mutationFn: () => pipelinesApi.execute(id!),
    onSuccess: (run) => {
      addToast({ type: 'success', message: 'Pipeline started' })
      navigate(`/pipelines/${id}/run/${run.id}`)
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to execute pipeline' })
    },
  })

  useEffect(() => {
    if (pipeline?.stages) {
      setStages(pipeline.stages)
    }
  }, [pipeline])

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  if (!pipeline) {
    return <div className={styles.container}>Pipeline not found</div>
  }

  const runs: PipelineRun[] = runsData?.runs ?? []

  const getStageStatus = (runs: PipelineRun[]) => {
    const lastRun = runs[0]
    if (!lastRun) return 'pending'
    if (lastRun.status === 'success') return 'completed'
    if (lastRun.status === 'running') return 'running'
    if (lastRun.status === 'failed') return 'failed'
    return 'pending'
  }

  const renderStageIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <Check size={16} className={styles.stageIconCompleted} />
      case 'running':
        return <Loader size={16} className={styles.stageIconRunning} />
      case 'failed':
        return <X size={16} className={styles.stageIconFailed} />
      default:
        return <div className={styles.stageIconPending} />
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/pipelines')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{pipeline.name}</h1>
        <div className={styles.actions}>
          <Button
            variant="primary"
            onClick={() => executeMutation.mutate()}
            disabled={pipeline.status === 'running'}
          >
            <Play size={16} />
            Execute
          </Button>
        </div>
      </div>

      <div className={styles.stagesSection}>
        <h2 className={styles.sectionTitle}>Stages</h2>
        <div className={styles.stagesContainer}>
          {stages.length === 0 ? (
            <div className={styles.emptyRuns}>No stages configured</div>
          ) : (
            stages.map((stage, index) => {
              const stageStatus = getStageStatus(runs)
              return (
                <div key={index}>
                  <div className={`${styles.stage} ${styles[`stage${stageStatus.charAt(0).toUpperCase() + stageStatus.slice(1)}`]}`}>
                    <div className={styles.stageIcon}>
                      {renderStageIcon(stageStatus)}
                    </div>
                    <span className={styles.stageName}>{stage}</span>
                  </div>
                  {index < stages.length - 1 && (
                    <div className={`${styles.connector} ${stageStatus === 'completed' ? styles.connectorCompleted : ''}`} />
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>

      <div className={styles.runsSection}>
        <h2 className={styles.sectionTitle}>Run History</h2>
        {runs.length === 0 ? (
          <div className={styles.emptyRuns}>No runs yet</div>
        ) : (
          <table className={styles.runsTable}>
            <thead className={styles.runsTableHeader}>
              <tr>
                <th className={styles.runsTableHeaderCell}>Run ID</th>
                <th className={styles.runsTableHeaderCell}>Status</th>
                <th className={styles.runsTableHeaderCell}>Started</th>
                <th className={styles.runsTableHeaderCell}>Finished</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <tr key={run.id} className={styles.runsTableRow}>
                  <td className={styles.runsTableCell}>{run.id.slice(0, 8)}</td>
                  <td className={styles.runsTableCell}>
                    <span className={styles.runStatus}>
                      <span className={`${styles.statusDot} ${styles[`status${run.status.charAt(0).toUpperCase() + run.status.slice(1)}`]}`} />
                      {run.status}
                    </span>
                  </td>
                  <td className={styles.runsTableCell}>{formatDate(run.startedAt)}</td>
                  <td className={styles.runsTableCell}>{formatDate(run.finishedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}