import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronRight, FolderTree } from 'lucide-react'
import { apiClient } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './ProjectList.module.css'

interface Project {
  id: string
  name: string
  type?: string
  description?: string
}

interface System {
  id: string
  name: string
  projects: Project[]
}

interface BusinessLine {
  id: string
  name: string
  systems: System[]
}

interface ProjectTreeResponse {
  businessLines: BusinessLine[]
}

export function ProjectList() {
  const navigate = useNavigate()
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set())

  const { data, isLoading } = useQuery({
    queryKey: ['projects', 'tree'],
    queryFn: () => apiClient.get<ProjectTreeResponse>('/api/v1/projects/tree'),
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

  const businessLines: BusinessLine[] = data?.businessLines ?? []

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Projects</h1>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading projects...</div>
      ) : businessLines.length === 0 ? (
        <EmptyState
          title="No projects found"
          description="Get started by creating your first project"
        />
      ) : (
        <Card className={styles.treeContainer}>
          {businessLines.map((bl) => (
            <div key={bl.id} className={styles.treeLevel}>
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

              {expandedItems.has(bl.id) && bl.systems.map((system) => (
                <div key={system.id} className={styles.treeLevel}>
                  <div
                    className={styles.system}
                    onClick={() => toggleExpand(system.id)}
                  >
                    <span className={`${styles.expandIcon} ${expandedItems.has(system.id) ? styles.expandIconExpanded : ''}`}>
                      <ChevronRight size={14} />
                    </span>
                    <span>{system.name}</span>
                  </div>

                  {expandedItems.has(system.id) && system.projects.map((project) => (
                    <div
                      key={project.id}
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
                  ))}
                </div>
              ))}
            </div>
          ))}
        </Card>
      )}
    </div>
  )
}