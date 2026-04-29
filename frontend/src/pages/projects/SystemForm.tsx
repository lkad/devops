import { useState } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { projectsApi, type System, type CreateSystemRequest, type UpdateSystemRequest } from '@/api/endpoints/projects'

interface SystemFormProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
  businessLineId: string
  system?: System
}

export function SystemForm({ isOpen, onClose, onSuccess, businessLineId, system }: SystemFormProps) {
  const [name, setName] = useState(system?.name || '')
  const [description, setDescription] = useState(system?.description || '')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const isEditing = !!system

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
        const data: UpdateSystemRequest = { name, description }
        await projectsApi.updateSystem(system.id, data)
      } else {
        const data: CreateSystemRequest = { name, description }
        await projectsApi.createSystem(businessLineId, data)
      }
      onSuccess()
      handleClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save system')
    } finally {
      setLoading(false)
    }
  }

  const handleClose = () => {
    setName('')
    setDescription('')
    setError('')
    onClose()
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={isEditing ? 'Edit System' : 'Add System'}
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
            placeholder="Enter system name"
            autoFocus
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