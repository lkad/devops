import { forwardRef, type SelectHTMLAttributes } from 'react'
import styles from './Select.module.css'

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
  options: Array<{ value: string; label: string }>
}

const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, label, error, id, options, ...props }, ref) => {
    const selectId = id || label?.toLowerCase().replace(/\s+/g, '-')

    const selectClassNames = [
      styles.select,
      error ? styles.hasError : '',
      className,
    ].filter(Boolean).join(' ')

    return (
      <div className={styles.wrapper}>
        {label && (
          <label htmlFor={selectId} className={styles.label}>
            {label}
          </label>
        )}
        <div className={styles.selectWrapper}>
          <select
            ref={ref}
            id={selectId}
            className={selectClassNames}
            {...props}
          >
            {options.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <span className={styles.arrow}>
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M2.5 4.5L6 8L9.5 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </span>
        </div>
        {error && (
          <span className={styles.error}>{error}</span>
        )}
      </div>
    )
  }
)

Select.displayName = 'Select'

export { Select }