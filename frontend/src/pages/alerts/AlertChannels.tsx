import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, Send, Mail, Webhook, FileText } from 'lucide-react'
import { alertsApi, type AlertChannel, type AlertChannelType } from '@/api/endpoints/alerts'
import { AlertChannelForm } from './AlertChannelForm'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { useToast } from '@/components/ui/Toast'
import styles from './AlertChannels.module.css'

const channelIconClass: Record<AlertChannelType, string> = {
  slack: styles.channelIconSlack,
  webhook: styles.channelIconWebhook,
  email: styles.channelIconEmail,
  log: styles.channelIconLog,
}

export function AlertChannels() {
  const queryClient = useQueryClient()
  const { addToast } = useToast()

  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingChannel, setEditingChannel] = useState<AlertChannel | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['alert-channels'],
    queryFn: () => alertsApi.listChannels(),
  })

  const createMutation = useMutation({
    mutationFn: (data: Omit<AlertChannel, 'id' | 'createdAt'>) => alertsApi.createChannel(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel created' })
      closeForm()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to create channel' })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ name, data }: { name: string; data: Partial<AlertChannel> }) =>
      alertsApi.updateChannel(name, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel updated' })
      closeForm()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to update channel' })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => alertsApi.deleteChannel(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      addToast({ type: 'success', message: 'Channel deleted' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to delete channel' })
    },
  })

  const testMutation = useMutation({
    mutationFn: (name: string) => alertsApi.testChannel(name),
    onSuccess: () => {
      addToast({ type: 'success', message: 'Test message sent' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to send test' })
    },
  })

  const openForm = (channel?: AlertChannel) => {
    setEditingChannel(channel || null)
    setIsFormOpen(true)
  }

  const closeForm = () => {
    setIsFormOpen(false)
    setEditingChannel(null)
  }

  const handleSubmit = (formData: Omit<AlertChannel, 'id' | 'createdAt'>) => {
    if (editingChannel) {
      updateMutation.mutate({ name: editingChannel.name, data: formData })
    } else {
      createMutation.mutate(formData)
    }
  }

  const channels: AlertChannel[] = data?.channels ?? []

  const getIcon = (type: AlertChannelType) => {
    switch (type) {
      case 'slack':
        return (
          <svg viewBox="0 0 24 24" fill="currentColor" width="20" height="20">
            <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.527 2.527 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z"/>
          </svg>
        )
      case 'email':
        return <Mail size={20} />
      case 'log':
        return <FileText size={20} />
      default:
        return <Webhook size={20} />
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Alert Channels</h1>
        <Button variant="primary" onClick={() => openForm()}>
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
            <Card key={channel.name} className={styles.channelCard}>
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
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => testMutation.mutate(channel.name)}
                  disabled={testMutation.isPending}
                >
                  <Send size={14} />
                  Test
                </Button>
                <Button variant="ghost" size="sm" onClick={() => openForm(channel)}>
                  <Pencil size={16} />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => deleteMutation.mutate(channel.name)}
                  disabled={deleteMutation.isPending}
                >
                  <Trash2 size={16} />
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <AlertChannelForm
        isOpen={isFormOpen}
        onClose={closeForm}
        onSubmit={handleSubmit}
        editingChannel={editingChannel}
        isLoading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  )
}