import { cn } from '@/lib/utils'

export interface StatusBadgeProps {
  status: string
  className?: string
}

const statusConfig = {
  online: { variant: 'success', label: 'Online' },
  active: { variant: 'success', label: 'Active' },
  offline: { variant: 'error', label: 'Offline' },
  error: { variant: 'error', label: 'Error' },
  pending: { variant: 'warning', label: 'Pending' },
  warning: { variant: 'warning', label: 'Warning' },
  running: { variant: 'success', label: 'Running' },
  stopped: { variant: 'error', label: 'Stopped' },
  unknown: { variant: 'default', label: 'Unknown' },
}

const dotColors = {
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-error',
  info: 'bg-info',
  default: 'bg-text-muted',
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const normalizedStatus = status.toLowerCase()
  const config = statusConfig[normalizedStatus as keyof typeof statusConfig] || statusConfig.unknown
  const dotColor = dotColors[config.variant as keyof typeof dotColors] || dotColors.default

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium',
        getBadgeClasses(config.variant),
        className
      )}
    >
      <span className={cn('w-1.5 h-1.5 rounded-full', dotColor)} />
      {config.label}
    </span>
  )
}

function getBadgeClasses(variant: string): string {
  switch (variant) {
    case 'success':
      return 'bg-success/10 text-success border border-success/20'
    case 'warning':
      return 'bg-warning/10 text-warning border border-warning/20'
    case 'error':
      return 'bg-error/10 text-error border border-error/20'
    case 'info':
      return 'bg-info/10 text-info border border-info/20'
    default:
      return 'bg-surface-elevated text-text-secondary border border-border'
  }
}