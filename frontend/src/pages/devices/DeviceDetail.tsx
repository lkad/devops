import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Wrench, Pause, Trash2 } from 'lucide-react'
import { devicesApi } from '@/api/endpoints/devices'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { ConfirmDialog } from '@/components/ui/ConfirmDialog'
import { useToast } from '@/components/ui/Toast'
import styles from './DeviceDetail.module.css'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
      return 'success'
    case 'maintenance':
    case 'suspended':
      return 'warning'
    case 'retired':
      return 'error'
    default:
      return 'default'
  }
}

const formatDate = (date?: string): string => {
  if (!date) return 'N/A'
  return new Date(date).toLocaleString()
}

export function DeviceDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { addToast } = useToast()

  const [showMaintenanceDialog, setShowMaintenanceDialog] = useState(false)
  const [showSuspendDialog, setShowSuspendDialog] = useState(false)
  const [showRetireDialog, setShowRetireDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const { data: device, isLoading } = useQuery({
    queryKey: ['device', id],
    queryFn: () => devicesApi.get(id!),
    enabled: !!id,
  })

  const updateMutation = useMutation({
    mutationFn: (data: { status?: string }) => devicesApi.update(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['device', id] })
      addToast({ type: 'success', message: 'Device updated successfully' })
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to update device' })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => devicesApi.delete(id!),
    onSuccess: () => {
      addToast({ type: 'success', message: 'Device deleted successfully' })
      navigate('/devices')
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to delete device' })
    },
  })

  const handleStatusChange = (newStatus: string) => {
    updateMutation.mutate({ status: newStatus })
  }

  if (isLoading) {
    return <div className={styles.container}>Loading...</div>
  }

  if (!device) {
    return <div className={styles.container}>Device not found</div>
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={() => navigate('/devices')}>
          <ArrowLeft size={20} />
        </button>
        <h1 className={styles.title}>{device.name}</h1>
        <div className={styles.actions}>
          <Button variant="secondary" onClick={() => setShowMaintenanceDialog(true)}>
            <Wrench size={16} />
            Maintenance
          </Button>
          <Button variant="secondary" onClick={() => setShowSuspendDialog(true)}>
            <Pause size={16} />
            Suspend
          </Button>
          <Button variant="danger" onClick={() => setShowRetireDialog(true)}>
            Retire
          </Button>
        </div>
      </div>

      <div className={styles.content}>
        <Card className={styles.detailCard}>
          <div className={styles.detailGrid}>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>Status</span>
              <Badge variant={statusVariant(device.status)}>{device.status}</Badge>
            </div>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>Type</span>
              <span className={styles.detailValue}>{device.type}</span>
            </div>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>Data Center</span>
              <span className={styles.detailValue}>{device.dataCenter || 'N/A'}</span>
            </div>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>IP Address</span>
              <span className={`${styles.detailValue} ${styles.monoValue}`}>
                {device.ipAddress || 'N/A'}
              </span>
            </div>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>Last Seen</span>
              <span className={styles.detailValue}>{formatDate(device.lastSeen)}</span>
            </div>
            <div className={styles.detailItem}>
              <span className={styles.detailLabel}>Created</span>
              <span className={styles.detailValue}>{formatDate(device.registeredAt)}</span>
            </div>
          </div>
        </Card>

        <Card className={styles.dangerZone}>
          <h3 className={styles.dangerTitle}>Danger Zone</h3>
          <p className={styles.dangerDescription}>
            Deleting a device is permanent and cannot be undone.
          </p>
          <Button variant="danger" onClick={() => setShowDeleteDialog(true)}>
            <Trash2 size={16} />
            Delete Device
          </Button>
        </Card>
      </div>

      <ConfirmDialog
        isOpen={showMaintenanceDialog}
        onClose={() => setShowMaintenanceDialog(false)}
        onConfirm={() => {
          handleStatusChange('maintenance')
          setShowMaintenanceDialog(false)
        }}
        title="Set Maintenance Mode"
        message="This will put the device into maintenance mode. Are you sure?"
        confirmLabel="Set Maintenance"
      />

      <ConfirmDialog
        isOpen={showSuspendDialog}
        onClose={() => setShowSuspendDialog(false)}
        onConfirm={() => {
          handleStatusChange('suspended')
          setShowSuspendDialog(false)
        }}
        title="Suspend Device"
        message="This will suspend the device. Are you sure?"
        confirmLabel="Suspend"
      />

      <ConfirmDialog
        isOpen={showRetireDialog}
        onClose={() => setShowRetireDialog(false)}
        onConfirm={() => {
          handleStatusChange('retired')
          setShowRetireDialog(false)
        }}
        title="Retire Device"
        message="This will retire the device. This action cannot be undone. Are you sure?"
        confirmLabel="Retire"
        danger
      />

      <ConfirmDialog
        isOpen={showDeleteDialog}
        onClose={() => setShowDeleteDialog(false)}
        onConfirm={() => {
          deleteMutation.mutate()
          setShowDeleteDialog(false)
        }}
        title="Delete Device"
        message="This will permanently delete the device. This action cannot be undone. Are you sure?"
        confirmLabel="Delete"
        danger
      />
    </div>
  )
}