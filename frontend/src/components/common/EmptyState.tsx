// EmptyState — placeholder for an empty list.
import { ReactNode } from 'react';

export function EmptyState({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }) {
  return (
    <div style={{
      background: 'var(--color-surface)',
      border: '1px dashed var(--color-border)',
      borderRadius: 'var(--radius-md)',
      padding: 'var(--sp-8)',
      textAlign: 'center',
      color: 'var(--color-text-secondary)',
    }}>
      <div style={{ fontSize: 'var(--fs-h3)', color: 'var(--color-text)', marginBottom: 'var(--sp-2)' }}>{title}</div>
      {hint && <div style={{ fontSize: 'var(--fs-small)', marginBottom: 'var(--sp-4)' }}>{hint}</div>}
      {action}
    </div>
  );
}
