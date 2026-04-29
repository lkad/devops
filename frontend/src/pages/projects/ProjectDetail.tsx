import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { apiClient } from '@/api/client'
import { Card } from '@/components/ui/Card'
import styles from './ProjectDetail.module.css'

type TabType = 'resources' | 'permissions'

interface Project {
  id: string
  name: string
  type?: string
  description?: string
  businessLineName: string
  systemName: string
  createdAt: string
}

interface Resource {
  id: string
  type: string
  name: string
  weight: number
}

interface Permission {
  id: string
  userId: string
  userName: string
  role: string
  level: number
}

export function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<TabType>('resources')

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', id],
    queryFn: () => apiClient.get<Project>(`/api/v1/projects/${id}`),
  })

  const { data: resourcesData } = useQuery({
    queryKey: ['project', id, 'resources'],
    queryFn: () => apiClient.get<{ resources: Resource[] }>(`/api/v1/projects/${id}/resources`),
    enabled: activeTab === 'resources' && !!id,
  })

  const { data: permissionsData } = useQuery({
    queryKey: ['project', id, 'permissions'],
    queryFn: () => apiClient.get<{ permissions: Permission[] }>(`/api/v1/projects/${id}/permissions`),
    enabled: activeTab === 'permissions' && !!id,
  })

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  const projectData = project || {
    id: id || 'unknown',
    name: 'Sample Project',
    type: 'application',
    description: 'A sample project description',
    businessLineName: 'Platform',
    systemName: 'Infrastructure',
    createdAt: new Date().toISOString(),
  }

  const resources: Resource[] = resourcesData?.resources ?? [
    { id: '1', type: 'device', name: 'dev-server-01', weight: 30 },
    { id: '2', type: 'pipeline', name: 'build-pipeline', weight: 40 },
    { id: '3', type: 'host', name: 'prod-host-01', weight: 30 },
  ]

  const permissions: Permission[] = permissionsData?.permissions ?? [
    { id: '1', userId: 'u1', userName: 'admin', role: 'admin', level: 100 },
    { id: '2', userId: 'u2', userName: 'developer', role: 'editor', level: 50 },
  ]

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/projects')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{projectData.name}</h1>
      </div>

      <Card className={styles.infoCard}>
        <div className={styles.infoGrid}>
          <div>
            <div className={styles.infoLabel}>Type</div>
            <div className={styles.infoValue}>{projectData.type || 'N/A'}</div>
          </div>
          <div>
            <div className={styles.infoLabel}>Business Line</div>
            <div className={styles.infoValue}>{projectData.businessLineName}</div>
          </div>
          <div>
            <div className={styles.infoLabel}>System</div>
            <div className={styles.infoValue}>{projectData.systemName}</div>
          </div>
          <div>
            <div className={styles.infoLabel}>Created</div>
            <div className={styles.infoValue}>{new Date(projectData.createdAt).toLocaleDateString()}</div>
          </div>
        </div>
      </Card>

      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${activeTab === 'resources' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('resources')}
        >
          Resources
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'permissions' ? styles.tabActive : ''}`}
          onClick={() => setActiveTab('permissions')}
        >
          Permissions
        </button>
      </div>

      {activeTab === 'resources' && (
        <div>
          <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>Linked Resources</h3>
          <Card>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: 'var(--color-surface-elevated)' }}>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Resource</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Type</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Weight</th>
                </tr>
              </thead>
              <tbody>
                {resources.map((resource) => (
                  <tr key={resource.id} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-primary)' }}>{resource.name}</td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>{resource.type}</td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-primary)' }}>{resource.weight}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
          <div style={{ marginTop: '16px', color: 'var(--color-text-muted)', fontSize: '14px' }}>
            Total weight: {resources.reduce((sum, r) => sum + r.weight, 0)}% (should equal 100%)
          </div>
        </div>
      )}

      {activeTab === 'permissions' && (
        <div>
          <h3 style={{ marginBottom: '16px', color: 'var(--color-text-primary)' }}>Permissions</h3>
          <Card>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: 'var(--color-surface-elevated)' }}>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>User</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Role</th>
                  <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '14px', fontWeight: 500, color: 'var(--color-text-secondary)', borderBottom: '1px solid var(--color-border)' }}>Level</th>
                </tr>
              </thead>
              <tbody>
                {permissions.map((perm) => (
                  <tr key={perm.id} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-primary)' }}>{perm.userName}</td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>{perm.role}</td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-primary)' }}>{perm.level}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        </div>
      )}
    </div>
  )
}