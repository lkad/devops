import { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { projectsApi, type Project, type CreateProjectRequest, type UpdateProjectRequest } from '@/api/endpoints/projects'

interface ProjectFormProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
  systemId: string
  project?: Project
}

export function ProjectForm({ isOpen, onClose, onSuccess, systemId, project }: ProjectFormProps) {
  const [name, setName] = useState(project?.name || '')
  const [type, setType] = useState<'frontend' | 'backend'>(project?.type || 'frontend')
  const [description, setDescription] = useState(project?.description || '')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const isEditing = !!project

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!name.trim()) {
      setError('Name is required')
      return
    }

    setLoading(true)
    try {
      if (isEditing) {
        const data: UpdateProjectRequest = { name, type, description }
        await projectsApi.updateProject(project.id, data)
      } else {
        const data: CreateProjectRequest = { name, type, description }
        await projectsApi.createProject(systemId, data)
      }
      onSuccess()
      handleClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save project')
    } finally {
      setLoading(false)
    }
  }

  const handleClose = () => {
    setName('')
    setType('frontend')
    setDescription('')
    setError('')
    onClose()
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={isEditing ? 'Edit Project' : 'Add Project'}
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} loading={loading}>
            {isEditing ? 'Update' : 'Create'}
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          <Input
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={error && !name.trim() ? error : undefined}
            placeholder="Enter project name"
            autoFocus
          />
          <Select
            label="Type"
            value={type}
            onChange={(e) => setType(e.target.value as 'frontend' | 'backend')}
            options={[
              { value: 'frontend', label: 'Frontend' },
              { value: 'backend', label: 'Backend' },
            ]}
          />
          <div>
            <label style={{ display: 'block', marginBottom: 'var(--space-1)', fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Enter description (optional)"
              style={{
                width: '100%',
                minHeight: '80px',
                padding: 'var(--space-2) var(--space-3)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--color-border)',
                background: 'var(--color-surface-elevated)',
                color: 'var(--color-text-primary)',
                fontSize: 'var(--text-body)',
                resize: 'vertical',
              }}
            />
          </div>
        </div>
      </form>
    </Modal>
  )
}