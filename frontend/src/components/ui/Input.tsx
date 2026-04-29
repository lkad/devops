import { forwardRef, type InputHTMLAttributes } from 'react'
import styles from './Input.module.css'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-')

    const inputClassNames = [
      styles.input,
      error ? styles.hasError : '',
      className,
    ].filter(Boolean).join(' ')

    return (
      <div className={styles.wrapper}>
        {label && (
          <label htmlFor={inputId} className={styles.label}>
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={inputClassNames}
          {...props}
        />
        {error && (
          <span className={styles.error}>{error}</span>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

export { Input }