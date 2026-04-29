import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Plus } from 'lucide-react'
import { devicesApi, type Device, type CreateDeviceRequest } from '@/api/endpoints/devices'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { useToast } from '@/components/ui/Toast'
import styles from './DeviceForm.module.css'

interface LabelPair {
  key: string
  value: string
}

interface DeviceFormData {
  name: string
  type: string
  environment: string
  dataCenter: string
  ipAddress: string
  labels: LabelPair[]
}

interface DeviceFormProps {
  isOpen: boolean
  onClose: () => void
  device?: Device | null
}

const initialFormData: DeviceFormData = {
  name: '',
  type: 'physical_host',
  environment: 'dev',
  dataCenter: '',
  ipAddress: '',
  labels: [],
}

const typeOptions = [
  { value: 'physical_host', label: 'Physical Host' },
  { value: 'vm', label: 'Virtual Machine' },
  { value: 'network_device', label: 'Network Device' },
]

const environmentOptions = [
  { value: 'dev', label: 'Development' },
  { value: 'test', label: 'Testing' },
  { value: 'prod', label: 'Production' },
]

export function DeviceForm({ isOpen, onClose, device }: DeviceFormProps) {
  const queryClient = useQueryClient()
  const { addToast } = useToast()
  const [formData, setFormData] = useState<DeviceFormData>(initialFormData)
  const [errors, setErrors] = useState<Partial<Record<keyof DeviceFormData, string>>>({})

  const isEditMode = !!device

  useEffect(() => {
    if (device) {
      setFormData({
        name: device.name || '',
        type: device.type || 'physical_host',
        environment: device.environment || 'dev',
        dataCenter: device.dataCenter || '',
        ipAddress: device.ipAddress || '',
        labels: Object.entries(device.labels || {}).map(([key, value]) => ({ key, value })),
      })
    } else {
      setFormData(initialFormData)
    }
    setErrors({})
  }, [device, isOpen])

  const createMutation = useMutation({
    mutationFn: (data: CreateDeviceRequest) => devicesApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      addToast({ type: 'success', message: 'Device created successfully' })
      handleClose()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to create device' })
    },
  })

  const updateMutation = useMutation({
    mutationFn: (data: CreateDeviceRequest) => devicesApi.update(device!.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      queryClient.invalidateQueries({ queryKey: ['device', device!.id] })
      addToast({ type: 'success', message: 'Device updated successfully' })
      handleClose()
    },
    onError: () => {
      addToast({ type: 'error', message: 'Failed to update device' })
    },
  })

  const handleClose = () => {
    setFormData(initialFormData)
    setErrors({})
    onClose()
  }

  const validate = (): boolean => {
    const newErrors: Partial<Record<keyof DeviceFormData, string>> = {}

    if (!formData.name.trim()) {
      newErrors.name = 'Name is required'
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!validate()) return

    const labels: Record<string, string> = {}
    formData.labels.forEach(({ key, value }) => {
      if (key.trim()) {
        labels[key.trim()] = value
      }
    })

    const payload: CreateDeviceRequest = {
      name: formData.name.trim(),
      type: formData.type,
      environment: formData.environment,
      dataCenter: formData.dataCenter.trim() || undefined,
      ipAddress: formData.ipAddress.trim() || undefined,
      labels: Object.keys(labels).length > 0 ? labels : undefined,
    }

    if (isEditMode) {
      updateMutation.mutate(payload)
    } else {
      createMutation.mutate(payload)
    }
  }

  const handleAddLabel = () => {
    setFormData(prev => ({
      ...prev,
      labels: [...prev.labels, { key: '', value: '' }],
    }))
  }

  const handleRemoveLabel = (index: number) => {
    setFormData(prev => ({
      ...prev,
      labels: prev.labels.filter((_, i) => i !== index),
    }))
  }

  const handleLabelChange = (index: number, field: 'key' | 'value', value: string) => {
    setFormData(prev => ({
      ...prev,
      labels: prev.labels.map((label, i) =>
        i === index ? { ...label, [field]: value } : label
      ),
    }))
  }

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={isEditMode ? 'Edit Device' : 'Create Device'}
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleSubmit}
            loading={isSubmitting}
            disabled={isSubmitting}
          >
            {isEditMode ? 'Update' : 'Create'}
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit} className={styles.form}>
        <Input
          label="Name"
          value={formData.name}
          onChange={e => setFormData(prev => ({ ...prev, name: e.target.value }))}
          error={errors.name}
          required
          placeholder="e.g., web-server-01"
        />

        <div className={styles.row}>
          <Select
            label="Type"
            value={formData.type}
            onChange={e => setFormData(prev => ({ ...prev, type: e.target.value }))}
            options={typeOptions}
          />

          <Select
            label="Environment"
            value={formData.environment}
            onChange={e => setFormData(prev => ({ ...prev, environment: e.target.value }))}
            options={environmentOptions}
          />
        </div>

        <Input
          label="Data Center"
          value={formData.dataCenter}
          onChange={e => setFormData(prev => ({ ...prev, dataCenter: e.target.value }))}
          placeholder="e.g., us-east-1a"
        />

        <Input
          label="IP Address"
          value={formData.ipAddress}
          onChange={e => setFormData(prev => ({ ...prev, ipAddress: e.target.value }))}
          placeholder="e.g., 192.168.1.100"
        />

        <div className={styles.labelsSection}>
          <div className={styles.labelsHeader}>
            <span className={styles.labelsTitle}>Labels</span>
            <button
              type="button"
              onClick={handleAddLabel}
              className={styles.addLabelButton}
            >
              <Plus size={16} />
              Add Label
            </button>
          </div>

          {formData.labels.length === 0 ? (
            <p className={styles.noLabels}>No labels added yet</p>
          ) : (
            <div className={styles.labelsList}>
              {formData.labels.map((label, index) => (
                <div key={index} className={styles.labelRow}>
                  <Input
                    placeholder="Key"
                    value={label.key}
                    onChange={e => handleLabelChange(index, 'key', e.target.value)}
                    className={styles.labelKeyInput}
                  />
                  <span className={styles.labelSeparator}>=</span>
                  <Input
                    placeholder="Value"
                    value={label.value}
                    onChange={e => handleLabelChange(index, 'value', e.target.value)}
                    className={styles.labelValueInput}
                  />
                  <button
                    type="button"
                    onClick={() => handleRemoveLabel(index)}
                    className={styles.removeLabelButton}
                    aria-label="Remove label"
                  >
                    <X size={16} />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </form>
    </Modal>
  )
}