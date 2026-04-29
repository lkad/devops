import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { apiClient } from '@/api/client'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { DataTable } from '@/components/ui/DataTable'
import styles from './FinOpsReport.module.css'

interface CostEntry {
  id: string
  service: string
  environment: string
  cost: number
  trend: number
  period: string
}

const mockCostData: CostEntry[] = [
  { id: '1', service: 'Compute', environment: 'prod', cost: 12450.00, trend: 5.2, period: '2024-Q1' },
  { id: '2', service: 'Storage', environment: 'prod', cost: 3200.00, trend: -2.1, period: '2024-Q1' },
  { id: '3', service: 'Network', environment: 'prod', cost: 1800.00, trend: 1.5, period: '2024-Q1' },
  { id: '4', service: 'Database', environment: 'prod', cost: 5600.00, trend: 8.3, period: '2024-Q1' },
  { id: '5', service: 'Compute', environment: 'dev', cost: 2100.00, trend: -5.0, period: '2024-Q1' },
  { id: '6', service: 'Storage', environment: 'dev', cost: 450.00, trend: 0.0, period: '2024-Q1' },
]

export function FinOpsReport() {
  const [period, setPeriod] = useState('2024-Q1')

  const { data, isLoading } = useQuery({
    queryKey: ['finops', 'report', period],
    queryFn: () => apiClient.get<{ costs: CostEntry[]; total: number }>('/api/v1/finops/report', {
      params: { period },
    }),
  })

  const costs: CostEntry[] = data?.costs ?? mockCostData

  const totalCost = costs.reduce((sum, c) => sum + c.cost, 0)
  const avgTrend = costs.reduce((sum, c) => sum + c.trend, 0) / costs.length

  const exportToCsv = () => {
    const headers = ['Service', 'Environment', 'Cost', 'Trend', 'Period']
    const rows = costs.map(c => [c.service, c.environment, c.cost.toFixed(2), c.trend.toFixed(1), c.period])
    const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `finops-report-${period}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  const columns = [
    { id: 'service', header: 'Service', accessor: (row: CostEntry) => row.service },
    { id: 'environment', header: 'Environment', accessor: (row: CostEntry) => row.environment },
    {
      id: 'cost',
      header: 'Cost',
      accessor: (row: CostEntry) => `$${row.cost.toFixed(2)}`,
    },
    {
      id: 'trend',
      header: 'Trend %',
      accessor: (row: CostEntry) => (
        <span style={{ color: row.trend > 0 ? 'var(--color-error)' : 'var(--color-success)' }}>
          {row.trend > 0 ? '+' : ''}{row.trend.toFixed(1)}%
        </span>
      ),
    },
    { id: 'period', header: 'Period', accessor: (row: CostEntry) => row.period },
  ]

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>FinOps Report</h1>
        <div className={styles.controls}>
          <select
            value={period}
            onChange={(e) => setPeriod(e.target.value)}
            className={styles.periodSelect}
          >
            <option value="2024-Q1">2024 Q1</option>
            <option value="2024-Q2">2024 Q2</option>
            <option value="2024-Q3">2024 Q3</option>
            <option value="2024-Q4">2024 Q4</option>
          </select>
          <Button variant="secondary" onClick={exportToCsv}>
            <Download size={16} />
            Export CSV
          </Button>
        </div>
      </div>

      <div className={styles.summaryGrid}>
        <Card className={styles.summaryCard}>
          <div className={styles.summaryValue}>${totalCost.toFixed(2)}</div>
          <div className={styles.summaryLabel}>Total Cost</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.summaryValue}>{costs.length}</div>
          <div className={styles.summaryLabel}>Services</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.summaryValue}>{avgTrend > 0 ? '+' : ''}{avgTrend.toFixed(1)}%</div>
          <div className={styles.summaryLabel}>Avg Trend</div>
        </Card>
        <Card className={styles.summaryCard}>
          <div className={styles.summaryValue}>{(totalCost / 30).toFixed(2)}</div>
          <div className={styles.summaryLabel}>Daily Avg</div>
        </Card>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading report...</div>
      ) : (
        <div className={styles.tableContainer}>
          <DataTable data={costs} columns={columns} pageSize={10} />
        </div>
      )}
    </div>
  )
}