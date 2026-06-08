// Button — primary / secondary / danger.
import { CSSProperties, ReactNode } from 'react';

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost';

const base: CSSProperties = {
  fontFamily: 'inherit',
  fontSize: 'var(--fs-small)',
  fontWeight: 500,
  padding: '6px 14px',
  borderRadius: 'var(--radius-sm)',
  cursor: 'pointer',
  transition: 'background 0.1s, border-color 0.1s',
  border: '1px solid transparent',
  display: 'inline-flex',
  alignItems: 'center',
  gap: 'var(--sp-2)',
};

const styles: Record<Variant, CSSProperties> = {
  primary: { ...base, background: 'var(--color-primary)', color: 'var(--color-bg)' },
  secondary: { ...base, background: 'var(--color-surface-elevated)', color: 'var(--color-text)', border: '1px solid var(--color-border)' },
  danger: { ...base, background: 'var(--color-error)', color: 'white' },
  ghost: { ...base, background: 'transparent', color: 'var(--color-text-secondary)' },
};

export function Button({
  variant = 'secondary',
  disabled,
  onClick,
  children,
  type = 'button',
  style,
}: {
  variant?: Variant;
  disabled?: boolean;
  onClick?: () => void;
  children: ReactNode;
  type?: 'button' | 'submit';
  style?: CSSProperties;
}) {
  const s = { ...styles[variant], ...(disabled ? { opacity: 0.5, cursor: 'not-allowed' } : {}), ...style };
  return (
    <button type={type} disabled={disabled} onClick={onClick} style={s}>
      {children}
    </button>
  );
}
