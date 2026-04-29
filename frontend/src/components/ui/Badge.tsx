import { type HTMLAttributes } from 'react'
import styles from './Badge.module.css'

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'neutral' | 'success' | 'warning' | 'error' | 'info' | 'default'
}

const Badge = ({ className, variant = 'neutral', children, ...props }: BadgeProps) => {
  const effectiveVariant = variant === 'default' ? 'neutral' : variant
  const classNames = [
    styles.badge,
    styles[effectiveVariant],
    className,
  ].filter(Boolean).join(' ')

  return (
    <span className={classNames} {...props}>
      {children}
    </span>
  )
}

Badge.displayName = 'Badge'

export { Badge }