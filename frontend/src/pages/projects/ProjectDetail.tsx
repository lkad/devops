import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Pencil, Trash2, Plus, X } from 'lucide-react'
import { projectsApi, type Project, type Resource } from '@/api/endpoints/projects'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { ProjectForm } from './ProjectForm'
import styles from './ProjectDetail.module.css'

type TabType = 'resources' | 'permissions'

type ResourceType = 'device' | 'pipeline' | 'host'

interface LinkResourceModal {
  isOpen: boolean
  resourceType: ResourceType
  searchQuery: string
  selectedResourceId: string
  selectedResourceName: string
  weight: number
}

interface Permission {
  id: string
  userId: string
  userName: string
  role: string
  level: number
}

interface ProjectDetailData extends Project {
  businessLineName?: string
  systemName?: string
}

export function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<TabType>('resources')
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [linkModal, setLinkModal] = useState<LinkResourceModal>({
    isOpen: false,
    resourceType: 'device',
    searchQuery: '',
    selectedResourceId: '',
    selectedResourceName: '',
    weight: 1.0,
  })

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', id],
    queryFn: () => projectsApi.getProject(id!),
  })

  const { data: resourcesData } = useQuery({
    queryKey: ['project', id, 'resources'],
    queryFn: () => projectsApi.getProjectResources(id!),
    enabled: activeTab === 'resources' && !!id,
  })

  const { data: permissionsData } = useQuery({
    queryKey: ['project', id, 'permissions'],
    queryFn: () => projectsApi.getProjectPermissions(id!),
    enabled: activeTab === 'permissions' && !!id,
  })

  const deleteMutation = useMutation({
    mutationFn: () => projectsApi.deleteProject(id!),
    onSuccess: () => {
      navigate('/projects')
    },
  })

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  const projectData: ProjectDetailData = project || {
    id: id || 'unknown',
    name: 'Sample Project',
    type: 'frontend' as const,
    description: 'A sample project description',
    systemId: '',
    createdAt: new Date().toISOString(),
  }

  const resources: Resource[] = resourcesData?.data || []

  const permissions: Permission[] = permissionsData?.map((p, i) => ({
    id: String(i),
    userId: p.userId,
    userName: p.userId,
    role: p.role,
    level: p.role === 'owner' ? 100 : p.role === 'editor' ? 50 : 25,
  })) || []

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/projects')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{projectData.name}</h1>
        <div className={styles.headerActions}>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Pencil size={16} />}
            onClick={() => setEditModalOpen(true)}
          >
            Edit
          </Button>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Trash2 size={16} />}
            onClick={() => setDeleteDialogOpen(true)}
          >
            Delete
          </Button>
        </div>
      </div>

      <Card className={styles.infoCard}>
        <div className={styles.infoGrid}>
          <div>
            <div className={styles.infoLabel}>Type</div>
            <div className={styles.infoValue}>{projectData.type || 'N/A'}</div>
          </div>
          <div>
            <div className={styles.infoLabel}>Type</div>
            <div className={styles.infoValue}>{projectData.type || 'N/A'}</div>
          </div>
          <div>
            <div className={styles.infoLabel}>System ID</div>
            <div className={styles.infoValue}>{projectData.systemId || 'N/A'}</div>
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
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ color: 'var(--color-text-primary)', margin: 0 }}>Linked Resources</h3>
            <Button
              variant="secondary"
              size="sm"
              leftIcon={<Plus size={16} />}
              onClick={() => setLinkModal({ ...linkModal, isOpen: true })}
            >
              Link Resource
            </Button>
          </div>
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
                    <td style={{ padding: '12px 16px' }}>
                      <button
                        style={{
                          background: 'none',
                          border: 'none',
                          padding: 0,
                          cursor: 'pointer',
                          color: 'var(--color-primary)',
                          textDecoration: 'underline',
                          fontSize: '14px',
                        }}
                        onClick={() => {
                          if (resource.resource_type === 'device') navigate(`/devices/${resource.resource_id}`)
                          else if (resource.resource_type === 'physical_host') navigate(`/physical-hosts/${resource.resource_id}`)
                          else if (resource.resource_type === 'pipeline') navigate(`/pipelines/${resource.resource_id}`)
                        }}
                      >
                        {resource.resource_id}
                      </button>
                    </td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-secondary)' }}>{resource.resource_type}</td>
                    <td style={{ padding: '12px 16px', color: 'var(--color-text-primary)' }}>{(resource.weight * 100).toFixed(0)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
          <div style={{ marginTop: '16px', color: 'var(--color-text-muted)', fontSize: '14px' }}>
            Total weight: {(resources.reduce((sum, r) => sum + r.weight, 0) * 100).toFixed(0)}%
            {resources.reduce((sum, r) => sum + r.weight, 0) > 1 && (
              <span style={{ color: 'var(--color-error)', marginLeft: '8px' }}>Warning: Over 100%</span>
            )}
            {resources.reduce((sum, r) => sum + r.weight, 0) < 1 && resources.reduce((sum, r) => sum + r.weight, 0) > 0 && (
              <span style={{ color: 'var(--color-warning)', marginLeft: '8px' }}>Unallocated: {((1 - resources.reduce((sum, r) => sum + r.weight, 0)) * 100).toFixed(0)}%</span>
            )}
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

      <ProjectForm
        isOpen={editModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={() => queryClient.invalidateQueries({ queryKey: ['project', id] })}
        systemId={projectData.systemId}
        project={project as Project}
      />

      <ConfirmDialog
        isOpen={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        onConfirm={() => deleteMutation.mutate()}
        title="Delete Project"
        message={`Are you sure you want to delete "${projectData.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        loading={deleteMutation.isPending}
      />

      {/* Link Resource Modal */}
      {linkModal.isOpen && (
        <div className={styles.modalOverlay} onClick={() => setLinkModal({ ...linkModal, isOpen: false })}>
          <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h3 className={styles.modalTitle}>Link Resource</h3>
              <button
                className={styles.modalClose}
                onClick={() => setLinkModal({ ...linkModal, isOpen: false })}
              >
                <X size={18} />
              </button>
            </div>

            <div className={styles.modalBody}>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Resource Type</label>
                <div className={styles.resourceTypeButtons}>
                  {(['device', 'pipeline', 'host'] as ResourceType[]).map((type) => (
                    <button
                      key={type}
                      className={`${styles.typeButton} ${linkModal.resourceType === type ? styles.typeButtonActive : ''}`}
                      onClick={() => setLinkModal({ ...linkModal, resourceType: type, selectedResourceId: '', selectedResourceName: '' })}
                    >
                      {type === 'device' && 'Device'}
                      {type === 'pipeline' && 'Pipeline'}
                      {type === 'host' && 'Host'}
                    </button>
                  ))}
                </div>
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Weight (%)</label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  className={styles.formInput}
                  value={(linkModal.weight * 100) || ''}
                  onChange={(e) => setLinkModal({ ...linkModal, weight: (parseInt(e.target.value) || 0) / 100 })}
                  placeholder="100"
                />
              </div>

              <div className={styles.formHint}>
                Select a resource type, then click "Link" to add it.
                In production, this would open a search dialog to select the specific resource.
              </div>
            </div>

            <div className={styles.modalFooter}>
              <Button variant="ghost" onClick={() => setLinkModal({ ...linkModal, isOpen: false })}>
                Cancel
              </Button>
              <Button variant="primary" disabled>
                Link Resource
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}