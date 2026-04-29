import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import styles from './ConfigPushForm.module.css'

interface ConfigPushFormProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (content: string) => void
  currentConfig?: string
  isLoading?: boolean
}

export function ConfigPushForm({
  isOpen,
  onClose,
  onSubmit,
  currentConfig,
  isLoading,
}: ConfigPushFormProps) {
  const [configContent, setConfigContent] = useState('')

  useEffect(() => {
    setConfigContent(currentConfig || '')
  }, [currentConfig, isOpen])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(configContent)
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Push Configuration"
      footer={
        <div className={styles.footer}>
          <button
            type="button"
            className={styles.cancelButton}
            onClick={onClose}
            disabled={isLoading}
          >
            Cancel
          </button>
          <button
            type="submit"
            className={styles.submitButton}
            form="config-form"
            disabled={isLoading}
          >
            {isLoading ? 'Pushing...' : 'Push Config'}
          </button>
        </div>
      }
    >
      <form id="config-form" onSubmit={handleSubmit} className={styles.form}>
        {currentConfig && (
          <div className={styles.currentConfig}>
            <h4 className={styles.sectionTitle}>Current Configuration</h4>
            <pre className={styles.preview}>{currentConfig}</pre>
          </div>
        )}

        <div className={styles.field}>
          <label htmlFor="config-content" className={styles.label}>
            New Configuration
          </label>
          <textarea
            id="config-content"
            className={styles.textarea}
            value={configContent}
            onChange={e => setConfigContent(e.target.value)}
            placeholder="Enter configuration content..."
            rows={15}
            disabled={isLoading}
          />
          <span className={styles.hint}>
            Enter the configuration content to push to the host
          </span>
        </div>
      </form>
    </Modal>
  )
}