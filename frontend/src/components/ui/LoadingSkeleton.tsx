import styles from './LoadingSkeleton.module.css'

export interface LoadingSkeletonProps {
  variant?: 'text' | 'title' | 'avatar' | 'button' | 'card' | 'row'
  width?: string | number
  height?: string | number
  className?: string
}

export function LoadingSkeleton({
  variant = 'text',
  width,
  height,
  className,
}: LoadingSkeletonProps) {
  const style: React.CSSProperties = {}
  if (width) style.width = typeof width === 'number' ? `${width}px` : width
  if (height) style.height = typeof height === 'number' ? `${height}px` : height

  const variantClass = {
    text: styles.text,
    title: styles.title,
    avatar: styles.avatar,
    button: styles.button,
    card: styles.card,
    row: styles.row,
  }[variant]

  return (
    <div
      className={`${styles.skeleton} ${variantClass} ${className || ''}`}
      style={style}
      aria-hidden="true"
      data-testid="skeleton"
    />
  )
}
