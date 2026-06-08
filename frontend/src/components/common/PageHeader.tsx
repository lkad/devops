// PageContainer / PageHeader — convenience wrappers around the
// standard "page title + actions + body" pattern.
import { ReactNode } from 'react';

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: string; actions?: ReactNode }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 'var(--sp-5)', gap: 'var(--sp-4)' }}>
      <div>
        <h1 style={{ fontSize: 'var(--fs-h1)', marginBottom: subtitle ? 'var(--sp-1)' : 0 }}>{title}</h1>
        {subtitle && <div style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--fs-small)' }}>{subtitle}</div>}
      </div>
      {actions && <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>{actions}</div>}
    </div>
  );
}
