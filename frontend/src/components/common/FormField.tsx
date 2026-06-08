// FormField — labeled input wrapper.
import { ReactNode } from 'react';

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: 'var(--fs-caption)',
  color: 'var(--color-text-secondary)',
  marginBottom: 4,
  textTransform: 'uppercase',
  letterSpacing: 0.5,
  fontWeight: 600,
};

const inputStyle: React.CSSProperties = {
  width: '100%',
  background: 'var(--color-bg)',
  color: 'var(--color-text)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-sm)',
  padding: '6px 10px',
  fontSize: 'var(--fs-small)',
};

export function FormField({ label, children, hint }: { label: string; children: (s: React.CSSProperties) => ReactNode; hint?: string }) {
  return (
    <div style={{ marginBottom: 'var(--sp-4)' }}>
      <label style={labelStyle}>{label}</label>
      {children(inputStyle)}
      {hint && <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', marginTop: 4 }}>{hint}</div>}
    </div>
  );
}
