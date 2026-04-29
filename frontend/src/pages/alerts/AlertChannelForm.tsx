import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import type { AlertChannel, AlertChannelType } from '@/api/endpoints/alerts'
import styles from './AlertChannelForm.module.css'

interface AlertChannelFormProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: Omit<AlertChannel, 'id' | 'createdAt'>) => void
  editingChannel?: AlertChannel | null
  isLoading?: boolean
}

const channelTypeOptions = [
  { value: 'slack', label: 'Slack' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'email', label: 'Email' },
  { value: 'log', label: 'Log' },
]

export function AlertChannelForm({
  isOpen,
  onClose,
  onSubmit,
  editingChannel,
  isLoading = false,
}: AlertChannelFormProps) {
  const [name, setName] = useState('')
  const [type, setType] = useState<AlertChannelType>('webhook')
  const [enabled, setEnabled] = useState(true)
  const [errors, setErrors] = useState<Record<string, string>>({})

  // Slack fields
  const [webhookUrl, setWebhookUrl] = useState('')
  const [slackChannel, setSlackChannel] = useState('')

  // Webhook fields
  const [url, setUrl] = useState('')
  const [headers, setHeaders] = useState('{}')

  // Email fields
  const [recipients, setRecipients] = useState('')
  const [smtpHost, setSmtpHost] = useState('')
  const [smtpPort, setSmtpPort] = useState('')

  useEffect(() => {
    if (isOpen) {
      if (editingChannel) {
        setName(editingChannel.name)
        setType(editingChannel.type)
        setEnabled(editingChannel.enabled)
        setErrors({})

        // Parse config based on type
        const config = editingChannel.config
        switch (editingChannel.type) {
          case 'slack':
            setWebhookUrl(config.webhookUrl || '')
            setSlackChannel(config.channel || '')
            break
          case 'webhook':
            setUrl(config.url || '')
            setHeaders(config.headers || '{}')
            break
          case 'email':
            setRecipients(config.recipients || '')
            setSmtpHost(config.smtpHost || '')
            setSmtpPort(config.smtpPort || '')
            break
          case 'log':
          default:
            break
        }
      } else {
        resetForm()
      }
    }
  }, [isOpen, editingChannel])

  const resetForm = () => {
    setName('')
    setType('webhook')
    setEnabled(true)
    setWebhookUrl('')
    setSlackChannel('')
    setUrl('')
    setHeaders('{}')
    setRecipients('')
    setSmtpHost('')
    setSmtpPort('')
    setErrors({})
  }

  const buildConfig = (): Record<string, string> => {
    switch (type) {
      case 'slack':
        return {
          webhookUrl,
          channel: slackChannel,
        }
      case 'webhook':
        return {
          url,
          headers,
        }
      case 'email':
        return {
          recipients,
          smtpHost,
          smtpPort,
        }
      case 'log':
      default:
        return {}
    }
  }

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {}

    if (!name.trim()) {
      newErrors.name = 'Name is required'
    }

    switch (type) {
      case 'slack':
        if (!webhookUrl.trim()) {
          newErrors.webhookUrl = 'Webhook URL is required'
        }
        break
      case 'webhook':
        if (!url.trim()) {
          newErrors.url = 'URL is required'
        }
        try {
          JSON.parse(headers)
        } catch {
          newErrors.headers = 'Headers must be valid JSON'
        }
        break
      case 'email':
        if (!recipients.trim()) {
          newErrors.recipients = 'Recipients is required'
        }
        if (!smtpHost.trim()) {
          newErrors.smtpHost = 'SMTP host is required'
        }
        if (!smtpPort.trim()) {
          newErrors.smtpPort = 'SMTP port is required'
        }
        break
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = () => {
    if (!validate()) return

    onSubmit({
      name: name.trim(),
      type,
      config: buildConfig(),
      enabled,
    })
  }

  const renderTypeSpecificFields = () => {
    switch (type) {
      case 'slack':
        return (
          <>
            <Input
              label="Webhook URL"
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              error={errors.webhookUrl}
              placeholder="https://hooks.slack.com/services/..."
            />
            <Input
              label="Channel"
              type="text"
              value={slackChannel}
              onChange={(e) => setSlackChannel(e.target.value)}
              placeholder="#alerts"
            />
          </>
        )
      case 'webhook':
        return (
          <>
            <Input
              label="URL"
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              error={errors.url}
              placeholder="https://example.com/webhook"
            />
            <div className={styles.formField}>
              <label className={styles.formLabel}>Headers (JSON)</label>
              <textarea
                className={`${styles.textarea} ${errors.headers ? styles.hasError : ''}`}
                value={headers}
                onChange={(e) => setHeaders(e.target.value)}
                rows={3}
                placeholder='{"Authorization": "Bearer token"}'
              />
              {errors.headers && <span className={styles.error}>{errors.headers}</span>}
            </div>
          </>
        )
      case 'email':
        return (
          <>
            <Input
              label="Recipients (comma-separated)"
              type="text"
              value={recipients}
              onChange={(e) => setRecipients(e.target.value)}
              error={errors.recipients}
              placeholder="user1@example.com, user2@example.com"
            />
            <Input
              label="SMTP Host"
              type="text"
              value={smtpHost}
              onChange={(e) => setSmtpHost(e.target.value)}
              error={errors.smtpHost}
              placeholder="smtp.example.com"
            />
            <Input
              label="SMTP Port"
              type="text"
              value={smtpPort}
              onChange={(e) => setSmtpPort(e.target.value)}
              error={errors.smtpPort}
              placeholder="587"
            />
          </>
        )
      case 'log':
        return (
          <p className={styles.noFieldsNote}>
            Log channels will forward alerts to the configured log storage.
            No additional configuration required.
          </p>
        )
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={editingChannel ? 'Edit Channel' : 'Create Channel'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleSubmit} loading={isLoading}>
            {editingChannel ? 'Update' : 'Create'}
          </Button>
        </>
      }
    >
      <div className={styles.form}>
        <div className={styles.formGrid}>
          <Input
            label="Name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={errors.name}
            placeholder="My Alert Channel"
          />

          <Select
            label="Type"
            value={type}
            onChange={(e) => setType(e.target.value as AlertChannelType)}
            options={channelTypeOptions}
          />
        </div>

        <div className={styles.typeSpecificFields}>
          {renderTypeSpecificFields()}
        </div>

        <div className={styles.checkboxField}>
          <label className={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className={styles.checkbox}
            />
            <span>Enabled</span>
          </label>
        </div>
      </div>
    </Modal>
  )
}