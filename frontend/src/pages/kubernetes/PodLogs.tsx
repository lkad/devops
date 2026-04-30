import { useState, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, RefreshCw, Download, Play, History } from 'lucide-react'
import { kubernetesApi, K8sApiResponse, K8sPodLogsResponse } from '@/api/endpoints/kubernetes'
import { Button } from '@/components/ui/Button'
import styles from './PodLogs.module.css'

type LogMode = 'realtime' | 'historical'

export function PodLogs() {
  const { cluster, namespace, pod } = useParams<{ cluster: string; namespace: string; pod: string }>()
  const navigate = useNavigate()
  const [mode, setMode] = useState<LogMode>('realtime')
  const [lineCount, setLineCount] = useState(100)
  const [refreshKey, setRefreshKey] = useState(0)
  const [historicalParams, setHistoricalParams] = useState({
    start: '',
    end: '',
    limit: 100,
  })
  const logsEndRef = useRef<HTMLDivElement>(null)

  // Real-time logs query
  const { data: realtimeLogs, isLoading: realtimeLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', 'logs', 'realtime', lineCount, refreshKey],
    queryFn: async () => {
      const response = await fetch(`/api/k8s/clusters/${cluster}/namespaces/${namespace}/pods/${pod}/logs?lines=${lineCount}`)
      if (!response.ok) {
        throw new Error('Failed to fetch logs')
      }
      return response.text()
    },
    enabled: mode === 'realtime' && !!cluster && !!namespace && !!pod,
    refetchInterval: 5000,
  })

  // Historical logs query
  const { data: historicalResponse, isLoading: historicalLoading } = useQuery<K8sApiResponse<K8sPodLogsResponse>>({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', 'logs', 'historical', historicalParams, refreshKey],
    queryFn: () => kubernetesApi.getPodLogsHistorical(cluster!, namespace!, pod!, {
      start: historicalParams.start || undefined,
      end: historicalParams.end || undefined,
      limit: historicalParams.limit.toString(),
    }),
    enabled: mode === 'historical' && !!cluster && !!namespace && !!pod,
  })

  const handleRefresh = () => {
    setRefreshKey(k => k + 1)
  }

  const handleDownload = () => {
    const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.data?.logs?.join('\n') || '')
    if (!logs) return
    const blob = new Blob([logs], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${pod}-${namespace}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleBack = () => {
    navigate(`/k8s/${cluster}`)
  }

  const switchToHistorical = () => {
    if (!historicalParams.start) {
      const now = new Date()
      const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000)
      setHistoricalParams({
        start: oneHourAgo.toISOString(),
        end: now.toISOString(),
        limit: 100,
      })
    }
    setMode('historical')
  }

  if (!cluster || !namespace || !pod) {
    return <div className={styles.container}>Pod not found</div>
  }

  const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.data?.logs?.join('\n') || '')
  const isLoading = mode === 'realtime' ? realtimeLoading : historicalLoading
  const backend = historicalResponse?.data?.backend || 'k8s-native'

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={handleBack}>
          <ArrowLeft size={20} />
        </button>
        <div className={styles.titleContainer}>
          <h1 className={styles.title}>{pod}</h1>
          <span className={styles.subtitle}>{namespace} / {cluster}</span>
        </div>
        <div className={styles.actions}>
          <Button variant="secondary" onClick={handleRefresh}>
            <RefreshCw size={16} />
            Refresh
          </Button>
          <Button variant="secondary" onClick={handleDownload} disabled={!logs}>
            <Download size={16} />
            Download
          </Button>
        </div>
      </div>

      {/* Mode Tabs */}
      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${mode === 'realtime' ? styles.tabActive : ''}`}
          onClick={() => setMode('realtime')}
        >
          <Play size={14} />
          Real-time
        </button>
        <button
          className={`${styles.tab} ${mode === 'historical' ? styles.tabActive : ''}`}
          onClick={() => switchToHistorical()}
        >
          <History size={14} />
          Historical
        </button>
      </div>

      {/* Mode-specific controls */}
      {mode === 'realtime' ? (
        <div className={styles.controls}>
          <div className={styles.lineCountControl}>
            <label>Lines:</label>
            <select
              value={lineCount}
              onChange={(e) => setLineCount(Number(e.target.value))}
              className={styles.lineSelect}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
              <option value={1000}>1000</option>
            </select>
          </div>
          <span className={styles.autoRefresh}>Auto-refresh: 5s</span>
        </div>
      ) : (
        <div className={styles.controls}>
          <div className={styles.timeRangeControl}>
            <label>From:</label>
            <input
              type="datetime-local"
              value={historicalParams.start?.slice(0, 16) || ''}
              onChange={(e) => setHistoricalParams(p => ({ ...p, start: new Date(e.target.value).toISOString() }))}
              className={styles.dateInput}
            />
            <label>To:</label>
            <input
              type="datetime-local"
              value={historicalParams.end?.slice(0, 16) || ''}
              onChange={(e) => setHistoricalParams(p => ({ ...p, end: new Date(e.target.value).toISOString() }))}
              className={styles.dateInput}
            />
          </div>
          <div className={styles.lineCountControl}>
            <label>Limit:</label>
            <select
              value={historicalParams.limit}
              onChange={(e) => setHistoricalParams(p => ({ ...p, limit: Number(e.target.value) }))}
              className={styles.lineSelect}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
              <option value={1000}>1000</option>
            </select>
          </div>
          <span className={styles.backendTag}>Backend: {backend}</span>
        </div>
      )}

      <div className={styles.logContainer}>
        {isLoading ? (
          <div className={styles.loading}>Loading logs...</div>
        ) : logs ? (
          <pre className={styles.logs}>{logs}</pre>
        ) : (
          <div className={styles.noLogs}>No logs available</div>
        )}
        <div ref={logsEndRef} />
      </div>
    </div>
  )
}