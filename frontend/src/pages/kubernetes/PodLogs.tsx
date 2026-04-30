import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, RefreshCw, Download } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import styles from './PodLogs.module.css'

export function PodLogs() {
  const { cluster, namespace, pod } = useParams<{ cluster: string; namespace: string; pod: string }>()
  const navigate = useNavigate()
  const [lineCount, setLineCount] = useState(100)
  const [refreshKey, setRefreshKey] = useState(0)

  const { data: logsResponse, isLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', pod, 'logs', lineCount, refreshKey],
    queryFn: async () => {
      const response = await fetch(`/api/k8s/clusters/${cluster}/namespaces/${namespace}/pods/${pod}/logs?lines=${lineCount}`)
      if (!response.ok) {
        throw new Error('Failed to fetch logs')
      }
      return response.text()
    },
    enabled: !!cluster && !!namespace && !!pod,
  })

  const handleRefresh = () => {
    setRefreshKey(k => k + 1)
  }

  const handleDownload = () => {
    if (!logsResponse) return
    const blob = new Blob([logsResponse], { type: 'text/plain' })
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

  if (!cluster || !namespace || !pod) {
    return <div className={styles.container}>Pod not found</div>
  }

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
          <Button variant="secondary" onClick={handleRefresh}>
            <RefreshCw size={16} />
            Refresh
          </Button>
          <Button variant="secondary" onClick={handleDownload} disabled={!logsResponse}>
            <Download size={16} />
            Download
          </Button>
        </div>
      </div>

      <div className={styles.logContainer}>
        {isLoading ? (
          <div className={styles.loading}>Loading logs...</div>
        ) : logsResponse ? (
          <pre className={styles.logs}>{logsResponse}</pre>
        ) : (
          <div className={styles.noLogs}>No logs available</div>
        )}
      </div>
    </div>
  )
}