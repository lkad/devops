import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import styles from './Button.module.css'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'default'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  leftIcon?: ReactNode
  rightIcon?: ReactNode
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({
    className,
    variant = 'primary',
    size = 'md',
    loading = false,
    disabled,
    children,
    leftIcon,
    rightIcon,
    ...props
  }, ref) => {
    const classNames = [
      styles.button,
      styles[variant],
      styles[size],
      loading ? styles.loading : '',
      className,
    ].filter(Boolean).join(' ')

    return (
      <button
        ref={ref}
        className={classNames}
        disabled={disabled || loading}
        {...props}
      >
        {loading && <span className={styles.spinner} />}
        {leftIcon && !loading && leftIcon}
        {children}
        {rightIcon && !loading && rightIcon}
      </button>
    )
  }
)

Button.displayName = 'Button'

export { Button }