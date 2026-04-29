import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { ArrowLeft, Square, Check, Loader, X } from 'lucide-react'
import { pipelinesApi, type PipelineRun as PipelineRunType } from '@/api/endpoints/pipelines'
import { Button } from '@/components/ui/Button'
import { useWebSocket } from '@/hooks/useWebSocket'
import styles from './PipelineRun.module.css'

interface LogLine {
  timestamp: string
  level: string
  message: string
}

const formatTime = (date: Date): string => {
  return date.toLocaleTimeString('en-US', { hour12: false })
}

export function PipelineRun() {
  const { pipelineId, runId } = useParams<{ pipelineId: string; runId: string }>()
  const navigate = useNavigate()
  const ws = useWebSocket()
  const logsEndRef = useRef<HTMLDivElement>(null)

  const [logs, setLogs] = useState<LogLine[]>([])
  const [isConnected, setIsConnected] = useState(false)

  const { data: run, isLoading } = useQuery({
    queryKey: ['pipeline-run', runId],
    queryFn: () => pipelinesApi.getRun(runId!),
    enabled: !!runId,
    refetchInterval: (query) => {
      const run = query.state.data as PipelineRunType | undefined
      return run?.status === 'running' || run?.status === 'pending' ? 2000 : false
    },
  })

  useEffect(() => {
    if (ws) {
      ws.on('log', (data: unknown) => {
        const logData = data as { pipelineId?: string; message?: string; level?: string }
        if (logData.pipelineId === pipelineId && logData.message) {
          setLogs((prev) => [
            ...prev,
            {
              timestamp: new Date().toISOString(),
              level: logData.level || 'info',
              message: logData.message || '',
            },
          ])
        }
      })
      setIsConnected(true)
    }
    return () => {
      ws.off('log', () => {})
    }
  }, [ws, pipelineId])

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  const cancelMutation = useMutation({
    mutationFn: () => pipelinesApi.cancelRun(runId!),
    onSuccess: () => {
      navigate(`/pipelines/${pipelineId}`)
    },
    onError: () => {},
  })

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  if (!run) {
    return <div className={styles.container}>Run not found</div>
  }

  const stages = ['Build', 'Test', 'Deploy', 'Verify']

  const getStageIcon = (index: number) => {
    if (run.status === 'success' || run.status === 'cancelled') return <Check size={14} />
    if (run.status === 'failed') return <X size={14} />
    if (run.status === 'running' && index === 0) return <Loader size={14} />
    return null
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate(`/pipelines/${pipelineId}`)}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>Pipeline Run</h1>
        <div className={styles.actions}>
          {run.status === 'running' || run.status === 'pending' ? (
            <Button variant="danger" onClick={() => cancelMutation.mutate()}>
              <Square size={16} />
              Cancel
            </Button>
          ) : (
            <Button variant="secondary" onClick={() => navigate(`/pipelines/${pipelineId}`)}>
              Back to Pipeline
            </Button>
          )}
        </div>
      </div>

      <div className={styles.progressSection}>
        <div className={styles.stagesContainer}>
          {stages.map((stage, index) => (
            <div key={stage} className={styles.stage}>
              <div className={`${styles.stageIcon} ${
                run.status === 'success' ? styles.stageIconCompleted :
                run.status === 'failed' ? styles.stageIconFailed :
                run.status === 'running' && index === 0 ? styles.stageIconRunning :
                styles.stageIconPending
              }`}>
                {getStageIcon(index)}
              </div>
              <span className={styles.stageName}>{stage}</span>
            </div>
          ))}
        </div>
      </div>

      <div className={styles.logsSection}>
        <div className={styles.logsHeader}>
          <h2 className={styles.logsTitle}>Logs</h2>
          {isConnected && run.status === 'running' && (
            <span className={styles.liveIndicator}>
              <span className={styles.liveDot} />
              Live
            </span>
          )}
        </div>
        <div className={styles.logsContainer}>
          {logs.length === 0 ? (
            <div className={styles.emptyLogs}>
              {run.status === 'running' ? 'Waiting for logs...' : 'No logs available'}
            </div>
          ) : (
            logs.map((log, index) => (
              <div key={index} className={styles.logLine}>
                <span className={styles.logTimestamp}>[{formatTime(new Date(log.timestamp))}]</span>
                <span className={`${styles.logLevel} ${styles[`logLevel${log.level.charAt(0).toUpperCase() + log.level.slice(1)}`]}`}>
                  {log.level.toUpperCase()}
                </span>
                <span className={styles.logMessage}>{log.message}</span>
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      </div>
    </div>
  )
}