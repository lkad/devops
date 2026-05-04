import { useState, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import DatePicker from 'react-datepicker'
import 'react-datepicker/dist/react-datepicker.css'
import { ArrowLeft, RefreshCw, Download, Play, History } from 'lucide-react'
import { kubernetesApi, K8sPodLogsResponse } from '@/api/endpoints/kubernetes'
import { Button } from '@/components/ui/Button'
import styles from './PodLogs.module.css'

type LogMode = 'realtime' | 'historical'

export function PodLogs() {
  const { cluster, namespace, pod } = useParams<{ cluster: string; namespace: string; pod: string }>()
  const navigate = useNavigate()
  const [mode, setMode] = useState<LogMode>('realtime')
  const [lineCount, setLineCount] = useState(100)
  const [refreshKey, setRefreshKey] = useState(0)
  const [historicalStartDate, setHistoricalStartDate] = useState(() => {
    const now = new Date()
    return new Date(now.getTime() - 60 * 60 * 1000)
  })
  const [historicalEndDate, setHistoricalEndDate] = useState(() => new Date())
  const logsEndRef = useRef<HTMLDivElement>(null)

  // Handle start date change - only adjust internal state, don't query
  const handleStartDateChange = (date: Date | null) => {
    if (!date) return
    const maxRange = 30 * 24 * 60 * 60 * 1000 // 30 days in ms
    const newEndTime = date.getTime() + maxRange
    if (historicalEndDate.getTime() > newEndTime) {
      setHistoricalEndDate(new Date(newEndTime))
    }
    setHistoricalStartDate(date)
  }

  // Handle end date change - only adjust internal state, don't query
  const handleEndDateChange = (date: Date | null) => {
    if (!date) return
    const maxRange = 30 * 24 * 60 * 60 * 1000 // 30 days in ms
    const newStartTime = date.getTime() - maxRange
    if (historicalStartDate.getTime() < newStartTime) {
      setHistoricalStartDate(new Date(newStartTime))
    }
    setHistoricalEndDate(date)
  }

  // Manual search - only triggers one query
  const handleSearch = () => {
    setRefreshKey(k => k + 1)
  }

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

  // Historical logs query - only triggers when refreshKey changes (user clicks Search or Refresh)
  const { data: historicalResponse, isLoading: historicalLoading } = useQuery<K8sPodLogsResponse>({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', 'logs', 'historical', historicalStartDate.getTime(), historicalEndDate.getTime(), refreshKey],
    queryFn: () => kubernetesApi.getPodLogsHistorical(cluster!, namespace!, pod!, {
      start: historicalStartDate.toISOString(),
      end: historicalEndDate.toISOString(),
      limit: '100',
    }),
    enabled: mode === 'historical' && !!cluster && !!namespace && !!pod,
  })

  const handleRefresh = () => {
    setRefreshKey(k => k + 1)
  }

  const handleDownload = () => {
    const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.logs?.join('\n') || '')
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
    setMode('historical')
  }

  if (!cluster || !namespace || !pod) {
    return <div className={styles.container}>Pod not found</div>
  }

  const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.logs?.join('\n') || '')
  const isLoading = mode === 'realtime' ? realtimeLoading : historicalLoading
  const backend = historicalResponse?.backend || 'k8s-native'

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
            <DatePicker
              selected={historicalStartDate}
              onChange={handleStartDateChange}
              showTimeSelect
              timeFormat="HH:mm"
              timeIntervals={1}
              dateFormat="yyyy-MM-dd HH:mm"
              className={styles.datePicker}
              maxDate={new Date()}
              selectsStart
              startDate={historicalStartDate}
              endDate={historicalEndDate}
            />
            <label>To:</label>
            <DatePicker
              selected={historicalEndDate}
              onChange={handleEndDateChange}
              showTimeSelect
              timeFormat="HH:mm"
              timeIntervals={1}
              dateFormat="yyyy-MM-dd HH:mm"
              className={styles.datePicker}
              maxDate={new Date()}
              selectsEnd
              startDate={historicalStartDate}
              endDate={historicalEndDate}
              minDate={historicalStartDate}
            />
            <Button variant="primary" onClick={handleSearch} size="sm">
              Search
            </Button>
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