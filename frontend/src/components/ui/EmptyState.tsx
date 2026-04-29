import { type ReactNode } from 'react'
import styles from './EmptyState.module.css'

export interface EmptyStateAction {
  label: string
  onClick: () => void
}

export interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: EmptyStateAction | ReactNode
  className?: string
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  const renderAction = () => {
    if (!action) return null
    if (typeof action === 'object' && 'label' in action && 'onClick' in action) {
      return (
        <button className={styles.actionButton} onClick={action.onClick}>
          {action.label}
        </button>
      )
    }
    return <div className={styles.action}>{action as ReactNode}</div>
  }

  return (
    <div className={`${styles.container} ${className || ''}`}>
      {icon && <div className={styles.icon}>{icon}</div>}
      <h3 className={styles.title}>{title}</h3>
      {description && <p className={styles.description}>{description}</p>}
      {renderAction()}
    </div>
  )
}