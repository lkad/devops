import { forwardRef, type HTMLAttributes } from 'react'
import styles from './Card.module.css'

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'elevated'
}

const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    const classNames = [
      styles.card,
      variant === 'elevated' ? styles.elevated : '',
      className,
    ].filter(Boolean).join(' ')

    return (
      <div ref={ref} className={classNames} {...props} />
    )
  }
)

Card.displayName = 'Card'

export { Card }