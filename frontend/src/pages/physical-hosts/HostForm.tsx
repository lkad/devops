import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import type { PhysicalHost, CreatePhysicalHostRequest } from '@/api/endpoints/physicalHosts'
import styles from './HostForm.module.css'

interface HostFormProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (data: CreatePhysicalHostRequest) => void
  host?: PhysicalHost
  isLoading?: boolean
}

export function HostForm({ isOpen, onClose, onSubmit, host, isLoading }: HostFormProps) {
  const [hostname, setHostname] = useState('')
  const [ip, setIp] = useState('')
  const [port, setPort] = useState(22)
  const [errors, setErrors] = useState<{ hostname?: string; ip?: string }>({})

  const isEdit = !!host

  useEffect(() => {
    if (host) {
      setHostname(host.hostname)
      setIp(host.ip)
      setPort(host.port)
    } else {
      setHostname('')
      setIp('')
      setPort(22)
    }
    setErrors({})
  }, [host, isOpen])

  const validate = (): boolean => {
    const newErrors: { hostname?: string; ip?: string } = {}

    if (!hostname.trim()) {
      newErrors.hostname = 'Hostname is required'
    }

    if (!ip.trim()) {
      newErrors.ip = 'IP address is required'
    } else {
      const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/
      if (!ipRegex.test(ip)) {
        newErrors.ip = 'Invalid IP address format'
      }
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!validate()) return

    onSubmit({ hostname: hostname.trim(), ip: ip.trim(), port })
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEdit ? 'Edit Host' : 'Add Host'}
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
            form="host-form"
            disabled={isLoading}
          >
            {isLoading ? 'Saving...' : isEdit ? 'Save Changes' : 'Add Host'}
          </button>
        </div>
      }
    >
      <form id="host-form" onSubmit={handleSubmit} className={styles.form}>
        <div className={styles.field}>
          <label htmlFor="hostname" className={styles.label}>
            Hostname <span className={styles.required}>*</span>
          </label>
          <input
            type="text"
            id="hostname"
            className={`${styles.input} ${errors.hostname ? styles.inputError : ''}`}
            value={hostname}
            onChange={e => setHostname(e.target.value)}
            placeholder="e.g., server01.example.com"
            disabled={isLoading}
          />
          {errors.hostname && <span className={styles.error}>{errors.hostname}</span>}
        </div>

        <div className={styles.field}>
          <label htmlFor="ip" className={styles.label}>
            IP Address <span className={styles.required}>*</span>
          </label>
          <input
            type="text"
            id="ip"
            className={`${styles.input} ${errors.ip ? styles.inputError : ''}`}
            value={ip}
            onChange={e => setIp(e.target.value)}
            placeholder="e.g., 192.168.1.100"
            disabled={isLoading}
          />
          {errors.ip && <span className={styles.error}>{errors.ip}</span>}
        </div>

        <div className={styles.field}>
          <label htmlFor="port" className={styles.label}>
            Port
          </label>
          <input
            type="number"
            id="port"
            className={styles.input}
            value={port}
            onChange={e => setPort(parseInt(e.target.value) || 22)}
            min={1}
            max={65535}
            disabled={isLoading}
          />
          <span className={styles.hint}>SSH port for connection (default: 22)</span>
        </div>
      </form>
    </Modal>
  )
}