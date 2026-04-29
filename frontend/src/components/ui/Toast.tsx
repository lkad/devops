import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import styles from './Toast.module.css'

export type ToastType = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
  id: string
  type: ToastType
  title?: string
  message: string
  duration?: number
}

interface ToastContextValue {
  toasts: Toast[]
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function useToast() {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error('useToast must be used within a ToastProvider')
  }
  return context
}

function ToastIcon({ type }: { type: ToastType }) {
  switch (type) {
    case 'success':
      return (
        <svg className={`${styles.icon} ${styles.iconSuccess}`} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M10 18a8 8 0 100-16 8 8 0 000 16z" stroke="currentColor" strokeWidth="1.5"/>
          <path d="M6 10l3 3 5-6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      )
    case 'error':
      return (
        <svg className={`${styles.icon} ${styles.iconError}`} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.5"/>
          <path d="M10 6v5M10 14h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
        </svg>
      )
    case 'warning':
      return (
        <svg className={`${styles.icon} ${styles.iconWarning}`} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M10 2L1.5 17h17L10 2z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          <path d="M10 8v4M10 15h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
        </svg>
      )
    case 'info':
      return (
        <svg className={`${styles.icon} ${styles.iconInfo}`} viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.5"/>
          <path d="M10 9v5M10 6h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
        </svg>
      )
  }
}

interface ToastItemProps {
  toast: Toast
  onRemove: (id: string) => void
}

function ToastItem({ toast, onRemove }: ToastItemProps) {
  const [exiting, setExiting] = useState(false)
  const duration = toast.duration ?? 3000

  useEffect(() => {
    const timer = setTimeout(() => {
      setExiting(true)
      setTimeout(() => onRemove(toast.id), 200)
    }, duration)
    return () => clearTimeout(timer)
  }, [toast.id, duration, onRemove])

  const handleClose = () => {
    setExiting(true)
    setTimeout(() => onRemove(toast.id), 200)
  }

  const borderClass = {
    success: styles.borderSuccess,
    error: styles.borderError,
    warning: styles.borderWarning,
    info: styles.borderInfo,
  }[toast.type]

  return (
    <div className={`${styles.toast} ${borderClass} ${exiting ? styles.exiting : ''}`} role="alert">
      <ToastIcon type={toast.type} />
      <div className={styles.content}>
        {toast.title && <div className={styles.title}>{toast.title}</div>}
        <div className={styles.message}>{toast.message}</div>
      </div>
      <button className={styles.closeButton} onClick={handleClose} aria-label="Close">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M11 3L3 11M3 3l8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
        </svg>
      </button>
    </div>
  )
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = Math.random().toString(36).substring(2, 9)
    setToasts((prev) => [...prev, { ...toast, id }])
  }, [])

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return (
    <ToastContext.Provider value={{ toasts, addToast, removeToast }}>
      {children}
      {createPortal(
        <div className={styles.container}>
          {toasts.map((toast) => (
            <ToastItem key={toast.id} toast={toast} onRemove={removeToast} />
          ))}
        </div>,
        document.body
      )}
    </ToastContext.Provider>
  )
}