import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, Send, Mail, Webhook, FileText } from 'lucide-react'
import { apiClient } from '@/api/client'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Modal } from '@/components/ui/Modal'
import { useToast } from '@/components/ui/Toast'
import styles from './AlertChannels.module.css'

type ChannelType = 'slack' | 'webhook' | 'email' | 'log'

interface AlertChannel {
  id: string
  name: string
  type: ChannelType
  config: Record<string, unknown>
  enabled: boolean
  createdAt: string
}

const channelIconClass = {
  slack: styles.channelIconSlack,
  webhook: styles.channelIconWebhook,
  email: styles.channelIconEmail,
  log: styles.channelIconLog,
}

export function AlertChannels() {
  const queryClient = useQueryClient()
  const { addToast } = useToast()

  const [isModalOpen, setIsModalOpen] = useState(false)
  const [editingChannel, setEditingChannel] = useState<AlertChannel | null>(null)
  const [formData, setFormData] = useState({
    name: '',
    type: 'webhook' as ChannelType,
    config: {} as Record<string, unknown>,
  })

  const { data, isLoading } = useQuery({
    queryKey: ['alert-channels'],
    queryFn: () => apiClient.get<{ channels: AlertChannel[] }>('/api/v1/alert-channels'),
  })

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => apiClient.post<AlertChannel>('/api/v1/alert-channels', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel created' })
      closeModal()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to create channel' })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: typeof formData }) =>
      apiClient.put<AlertChannel>(`/api/v1/alert-channels/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel updated' })
      closeModal()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to update channel' })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.delete(`/api/v1/alert-channels/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel deleted' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to delete channel' })
    },
  })

  const testMutation = useMutation({
    mutationFn: (id: string) => apiClient.post(`/api/v1/alert-channels/${id}/test`, {}),
    onSuccess: () => {
      addToast({ type: 'success', message: 'Test message sent' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to send test' })
    },
  })

  const openModal = (channel?: AlertChannel) => {
    if (channel) {
      setEditingChannel(channel)
      setFormData({
        name: channel.name,
        type: channel.type,
        config: channel.config,
      })
    } else {
      setEditingChannel(null)
      setFormData({ name: '', type: 'webhook', config: {} })
    }
    setIsModalOpen(true)
  }

  const closeModal = () => {
    setIsModalOpen(false)
    setEditingChannel(null)
  }

  const handleSubmit = () => {
    if (editingChannel) {
      updateMutation.mutate({ id: editingChannel.id, data: formData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const channels: AlertChannel[] = data?.channels ?? []

  const getIcon = (type: ChannelType) => {
    switch (type) {
      case 'slack': return <svg viewBox="0 0 24 24" fill="currentColor" width="20" height="20"><path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.527 2.527 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z"/></svg>
      case 'email': return <Mail size={20} />
      case 'log': return <FileText size={20} />
      default: return <Webhook size={20} />
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Alert Channels</h1>
        <Button variant="primary" onClick={() => openModal()}>
          <Plus size={18} />
          Add Channel
        </Button>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading channels...</div>
      ) : channels.length === 0 ? (
        <Card>
          <div className={styles.emptyState}>No alert channels configured</div>
        </Card>
      ) : (
        <div className={styles.channelsGrid}>
          {channels.map((channel) => (
            <Card key={channel.id} className={styles.channelCard}>
              <div className={styles.channelHeader}>
                <div className={`${styles.channelIcon} ${channelIconClass[channel.type]}`}>
                  {getIcon(channel.type)}
                </div>
                <div>
                  <h3 className={styles.channelName}>{channel.name}</h3>
                  <span className={styles.channelType}>{channel.type}</span>
                </div>
              </div>
              <div className={styles.channelStatus}>
                <span className={`${styles.statusDot} ${channel.enabled ? styles.statusActive : styles.statusInactive}`} />
                {channel.enabled ? 'Active' : 'Inactive'}
              </div>
              <div className={styles.channelActions}>
                <Button variant="secondary" size="sm" onClick={() => testMutation.mutate(channel.id)}>
                  <Send size={14} />
                  Test
                </Button>
                <Button variant="ghost" size="sm" onClick={() => openModal(channel)}>
                  <Pencil size={16} />
                </Button>
                <Button variant="ghost" size="sm" onClick={() => deleteMutation.mutate(channel.id)}>
                  <Trash2 size={16} />
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        isOpen={isModalOpen}
        onClose={closeModal}
        title={editingChannel ? 'Edit Channel' : 'Create Channel'}
      >
        <div className={styles.formSection}>
          <div className={styles.formGrid}>
            <div className={styles.formField}>
              <label className={styles.formLabel}>Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                className={styles.formInput}
              />
            </div>
            <div className={styles.formField}>
              <label className={styles.formLabel}>Type</label>
              <select
                value={formData.type}
                onChange={(e) => setFormData({ ...formData, type: e.target.value as ChannelType })}
                className={styles.formSelect}
              >
                <option value="slack">Slack</option>
                <option value="webhook">Webhook</option>
                <option value="email">Email</option>
                <option value="log">Log</option>
              </select>
            </div>
            <div className={`${styles.formField} ${styles.formFieldFull}`}>
              <label className={styles.formLabel}>Configuration (JSON)</label>
              <textarea
                value={JSON.stringify(formData.config, null, 2)}
                onChange={(e) => {
                  try {
                    setFormData({ ...formData, config: JSON.parse(e.target.value) })
                  } catch {}
                }}
                className={styles.formTextarea}
              />
            </div>
          </div>
          <div className={styles.formActions}>
            <Button variant="secondary" onClick={closeModal}>Cancel</Button>
            <Button variant="primary" onClick={handleSubmit}>
              {editingChannel ? 'Update' : 'Create'}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}