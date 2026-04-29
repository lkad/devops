import { type ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center py-12 px-4 text-center',
        className
      )}
    >
      {icon && (
        <div className="mb-4 text-text-muted">
          {icon}
        </div>
      )}
      <h3 className="text-lg font-medium text-text mb-1">
        {title}
      </h3>
      {description && (
        <p className="text-sm text-text-secondary max-w-sm mb-4">
          {description}
        </p>
      )}
      {action && (
        <div className="mt-2">
          {action}
        </div>
      )}
    </div>
  )
}