import type { ReactNode } from 'react'
import styles from './PageContainer.module.css'

interface PageContainerProps {
  title: string
  description?: string
  actions?: ReactNode
  children?: ReactNode
}

export function PageContainer({
  title,
  description,
  actions,
  children,
}: PageContainerProps) {
  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div className={styles.titleRow}>
          <h1 className={styles.title}>{title}</h1>
          {actions && <div className={styles.actions}>{actions}</div>}
        </div>
        {description && (
          <p className={styles.description}>{description}</p>
        )}
      </div>
      <div className={styles.content}>{children}</div>
    </div>
  )
}
