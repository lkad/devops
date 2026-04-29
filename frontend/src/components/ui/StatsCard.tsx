import { type ReactNode } from 'react'
import styles from './StatsCard.module.css'

export interface StatsCardProps {
  label: string
  value: string | number
  icon?: ReactNode
  iconColor?: string
  iconBg?: string
  trend?: {
    value: number
    label?: string
  }
  className?: string
}

function TrendArrow({ direction }: { direction: 'up' | 'down' | 'neutral' }) {
  if (direction === 'up') {
    return (
      <svg className={styles.trendIcon} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M8 12V4M8 4L4 8M8 4L12 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    )
  }
  if (direction === 'down') {
    return (
      <svg className={styles.trendIcon} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M8 4V12M8 12L4 8M8 12L12 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    )
  }
  return (
    <svg className={styles.trendIcon} viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M4 8H12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  )
}

export function StatsCard({
  label,
  value,
  icon,
  iconColor = 'var(--color-primary)',
  iconBg = 'var(--color-primary-muted)',
  trend,
  className,
}: StatsCardProps) {
  const trendDirection = trend
    ? trend.value > 0 ? 'up' : trend.value < 0 ? 'down' : 'neutral'
    : 'neutral'

  return (
    <div className={`${styles.card} ${className || ''}`}>
      <div className={styles.header}>
        {icon && (
          <div
            className={styles.iconWrapper}
            style={{ background: iconBg }}
          >
            <span style={{ color: iconColor }} className={styles.icon}>
              {icon}
            </span>
          </div>
        )}
      </div>
      <div className={styles.content}>
        <span className={styles.label}>{label}</span>
        <span className={styles.value}>{value}</span>
        {trend && (
          <span className={`${styles.trend} ${styles[`trend${trendDirection.charAt(0).toUpperCase() + trendDirection.slice(1)}`]}`}>
            <TrendArrow direction={trendDirection} />
            {Math.abs(trend.value)}%{trend.label && ` ${trend.label}`}
          </span>
        )}
      </div>
    </div>
  )
}

export function StatsGrid({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={`${styles.grid} ${className || ''}`}>
      {children}
    </div>
  )
}