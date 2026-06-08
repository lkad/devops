// DataTable — minimal, opinionated table. Header + body rows.
// For wider tables we wrap in a scroll container.
import { ReactNode } from 'react';
import { EmptyState } from './EmptyState';

export interface Column<T> {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
  width?: string | number;
  align?: 'left' | 'right' | 'center';
}

export function DataTable<T>({
  rows,
  columns,
  rowKey,
  empty,
  loading,
}: {
  rows: T[] | null;
  columns: Column<T>[];
  rowKey: (row: T) => string;
  empty: { title: string; hint?: string; action?: ReactNode };
  loading?: boolean;
}) {
  if (loading) return <div style={{ padding: 'var(--sp-6)', color: 'var(--color-text-muted)' }}>Loading…</div>;
  if (!rows || rows.length === 0) return <EmptyState title={empty.title} hint={empty.hint} action={empty.action} />;
  return (
    <div style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: 'var(--color-surface-elevated)' }}>
              {columns.map((c) => (
                <th
                  key={c.key}
                  style={{
                    textAlign: c.align ?? 'left',
                    padding: 'var(--sp-3) var(--sp-4)',
                    fontSize: 'var(--fs-caption)',
                    fontWeight: 600,
                    color: 'var(--color-text-secondary)',
                    borderBottom: '1px solid var(--color-border)',
                    width: c.width,
                    textTransform: 'uppercase',
                    letterSpacing: 0.5,
                  }}
                >{c.header}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={rowKey(row)} style={{ borderBottom: '1px solid var(--color-border-subtle)' }}>
                {columns.map((c) => (
                  <td
                    key={c.key}
                    style={{
                      padding: 'var(--sp-3) var(--sp-4)',
                      fontSize: 'var(--fs-small)',
                      textAlign: c.align ?? 'left',
                      verticalAlign: 'middle',
                    }}
                  >{c.render(row)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
