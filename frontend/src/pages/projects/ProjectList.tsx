import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ChevronRight, FolderTree, Plus, Pencil, Trash2 } from 'lucide-react'
import { projectsApi, type BusinessLine, type System, type Project } from '@/api/endpoints/projects'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { BusinessLineForm } from './BusinessLineForm'
import { SystemForm } from './SystemForm'
import { ProjectForm } from './ProjectForm'
import styles from './ProjectList.module.css'

export function ProjectList() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set())

  // Modal states
  const [blModalOpen, setBlModalOpen] = useState(false)
  const [systemModalOpen, setSystemModalOpen] = useState(false)
  const [projectModalOpen, setProjectModalOpen] = useState(false)
  const [editingBl, setEditingBl] = useState<BusinessLine | undefined>()
  const [editingSystem, setEditingSystem] = useState<System | undefined>()
  const [editingProject, setEditingProject] = useState<Project | undefined>()
  const [selectedBlId, setSelectedBlId] = useState<string>('')
  const [selectedSysId, setSelectedSysId] = useState<string>('')

  // Delete confirmation
  const [deleteTarget, setDeleteTarget] = useState<{ type: string; id: string; name: string } | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', 'tree'],
    queryFn: () => projectsApi.getProjectTree(),
  })

  const deleteMutation = useMutation({
    mutationFn: async ({ type, id }: { type: string; id: string }) => {
      switch (type) {
        case 'businessLine':
          return projectsApi.deleteBusinessLine(id)
        case 'system':
          return projectsApi.deleteSystem(id)
        case 'project':
          return projectsApi.deleteProject(id)
        default:
          throw new Error('Unknown type')
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', 'tree'] })
      setDeleteTarget(null)
    },
  })

  const toggleExpand = (id: string) => {
    const newExpanded = new Set(expandedItems)
    if (newExpanded.has(id)) {
      newExpanded.delete(id)
    } else {
      newExpanded.add(id)
    }
    setExpandedItems(newExpanded)
  }

  const handleProjectClick = (projectId: string) => {
    navigate(`/projects/${projectId}`)
  }

  const handleAddBusinessLine = () => {
    setEditingBl(undefined)
    setBlModalOpen(true)
  }

  const handleEditBusinessLine = (bl: BusinessLine, e: React.MouseEvent) => {
    e.stopPropagation()
    setEditingBl(bl)
    setBlModalOpen(true)
  }

  const handleDeleteBusinessLine = (bl: BusinessLine, e: React.MouseEvent) => {
    e.stopPropagation()
    setDeleteTarget({ type: 'businessLine', id: bl.id, name: bl.name })
  }

  const handleAddSystem = (blId: string) => {
    setSelectedBlId(blId)
    setEditingSystem(undefined)
    setSystemModalOpen(true)
  }

  const handleEditSystem = (sys: System, e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedBlId(sys.businessLineId)
    setEditingSystem(sys)
    setSystemModalOpen(true)
  }

  const handleDeleteSystem = (sys: System, e: React.MouseEvent) => {
    e.stopPropagation()
    setDeleteTarget({ type: 'system', id: sys.id, name: sys.name })
  }

  const handleAddProject = (sysId: string) => {
    setSelectedSysId(sysId)
    setEditingProject(undefined)
    setProjectModalOpen(true)
  }

  const handleEditProject = (proj: Project, e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedSysId(proj.systemId)
    setEditingProject(proj)
    setProjectModalOpen(true)
  }

  const handleDeleteProject = (proj: Project, e: React.MouseEvent) => {
    e.stopPropagation()
    setDeleteTarget({ type: 'project', id: proj.id, name: proj.name })
  }

  const businessLines: Array<BusinessLine & { systems: Array<System & { projects: Project[] }> }> =
    data?.businessLines ?? []

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Projects</h1>
        <Button leftIcon={<Plus size={16} />} onClick={handleAddBusinessLine}>
          Add Business Line
        </Button>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading projects...</div>
      ) : businessLines.length === 0 ? (
        <EmptyState
          title="No projects found"
          description="Get started by creating your first project"
          action={
            <Button onClick={handleAddBusinessLine}>Add Business Line</Button>
          }
        />
      ) : (
        <Card className={styles.treeContainer}>
          {businessLines.map((bl) => (
            <div key={bl.id} className={styles.treeLevel}>
              <div className={styles.treeItemRow}>
                <div
                  className={styles.businessLine}
                  onClick={() => toggleExpand(bl.id)}
                >
                  <span className={`${styles.expandIcon} ${expandedItems.has(bl.id) ? styles.expandIconExpanded : ''}`}>
                    <ChevronRight size={16} />
                  </span>
                  <FolderTree size={18} />
                  <span>{bl.name}</span>
                </div>
                <div className={styles.actions}>
                  <button
                    className={styles.actionButton}
                    onClick={() => handleAddSystem(bl.id)}
                    title="Add System"
                  >
                    <Plus size={14} />
                  </button>
                  <button
                    className={styles.actionButton}
                    onClick={(e) => handleEditBusinessLine(bl, e)}
                    title="Edit"
                  >
                    <Pencil size={14} />
                  </button>
                  <button
                    className={styles.actionButton}
                    onClick={(e) => handleDeleteBusinessLine(bl, e)}
                    title="Delete"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>

              {expandedItems.has(bl.id) && bl.systems.map((system) => (
                <div key={system.id} className={styles.treeLevel}>
                  <div className={styles.treeItemRow}>
                    <div
                      className={styles.system}
                      onClick={() => toggleExpand(system.id)}
                    >
                      <span className={`${styles.expandIcon} ${expandedItems.has(system.id) ? styles.expandIconExpanded : ''}`}>
                        <ChevronRight size={14} />
                      </span>
                      <span>{system.name}</span>
                    </div>
                    <div className={styles.actions}>
                      <button
                        className={styles.actionButton}
                        onClick={() => handleAddProject(system.id)}
                        title="Add Project"
                      >
                        <Plus size={14} />
                      </button>
                      <button
                        className={styles.actionButton}
                        onClick={(e) => handleEditSystem(system, e)}
                        title="Edit"
                      >
                        <Pencil size={14} />
                      </button>
                      <button
                        className={styles.actionButton}
                        onClick={(e) => handleDeleteSystem(system, e)}
                        title="Delete"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </div>

                  {expandedItems.has(system.id) && (system.projects ?? []).map((project) => (
                    <div key={project.id} className={styles.treeItemRow}>
                      <div
                        className={styles.project}
                        onClick={() => handleProjectClick(project.id)}
                      >
                        <span className={styles.projectName}>
                          <FolderTree size={14} />
                          <span>{project.name}</span>
                        </span>
                        {project.type && (
                          <span className={styles.projectType}>{project.type}</span>
                        )}
                      </div>
                      <div className={styles.actions}>
                        <button
                          className={styles.actionButton}
                          onClick={(e) => handleEditProject(project, e)}
                          title="Edit"
                        >
                          <Pencil size={14} />
                        </button>
                        <button
                          className={styles.actionButton}
                          onClick={(e) => handleDeleteProject(project, e)}
                          title="Delete"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ))}
        </Card>
      )}

      <BusinessLineForm
        isOpen={blModalOpen}
        onClose={() => setBlModalOpen(false)}
        onSuccess={() => queryClient.invalidateQueries({ queryKey: ['projects', 'tree'] })}
        businessLine={editingBl}
      />

      <SystemForm
        isOpen={systemModalOpen}
        onClose={() => setSystemModalOpen(false)}
        onSuccess={() => queryClient.invalidateQueries({ queryKey: ['projects', 'tree'] })}
        businessLineId={selectedBlId}
        system={editingSystem}
      />

      <ProjectForm
        isOpen={projectModalOpen}
        onClose={() => setProjectModalOpen(false)}
        onSuccess={() => queryClient.invalidateQueries({ queryKey: ['projects', 'tree'] })}
        systemId={selectedSysId}
        project={editingProject}
      />

      <ConfirmDialog
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => deleteTarget && deleteMutation.mutate({ type: deleteTarget.type, id: deleteTarget.id })}
        title="Confirm Delete"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        loading={deleteMutation.isPending}
      />
    </div>
  )
}