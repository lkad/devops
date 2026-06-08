// Modal — minimal centered overlay.
import { ReactNode } from 'react';

export function Modal({ open, onClose, title, children, footer, width = 480 }: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
}) {
  if (!open) return null;
  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-lg)',
          width,
          maxWidth: '90vw',
          maxHeight: '90vh',
          display: 'flex', flexDirection: 'column',
        }}
      >
        <div style={{ padding: 'var(--sp-4) var(--sp-5)', borderBottom: '1px solid var(--color-border)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h3 style={{ fontSize: 'var(--fs-h3)' }}>{title}</h3>
          <button onClick={onClose} style={{ background: 'transparent', border: 'none', color: 'var(--color-text-muted)', cursor: 'pointer', fontSize: 20, lineHeight: 1 }}>×</button>
        </div>
        <div style={{ padding: 'var(--sp-5)', overflow: 'auto' }}>{children}</div>
        {footer && <div style={{ padding: 'var(--sp-3) var(--sp-5)', borderTop: '1px solid var(--color-border)', display: 'flex', justifyContent: 'flex-end', gap: 'var(--sp-2)' }}>{footer}</div>}
      </div>
    </div>
  );
}
