// Badge — colored chip for status / state / env labels.
import { CSSProperties, ReactNode } from 'react';

type Tone = 'neutral' | 'success' | 'warning' | 'error' | 'info' | 'muted' | 'primary';

const tones: Record<Tone, CSSProperties> = {
  neutral: { background: 'var(--color-surface-elevated)', color: 'var(--color-text-secondary)', border: '1px solid var(--color-border)' },
  success: { background: 'rgba(34,197,94,0.15)', color: 'var(--color-success)', border: '1px solid rgba(34,197,94,0.3)' },
  warning: { background: 'rgba(245,158,11,0.15)', color: 'var(--color-warning)', border: '1px solid rgba(245,158,11,0.3)' },
  error: { background: 'rgba(239,68,68,0.15)', color: 'var(--color-error)', border: '1px solid rgba(239,68,68,0.3)' },
  info: { background: 'rgba(59,130,246,0.15)', color: 'var(--color-info)', border: '1px solid rgba(59,130,246,0.3)' },
  muted: { background: 'rgba(168,85,247,0.15)', color: '#a855f7', border: '1px solid rgba(168,85,247,0.3)' },
  primary: { background: 'var(--color-primary-muted)', color: 'var(--color-primary)', border: '1px solid rgba(34,211,238,0.3)' },
};

export function Badge({ tone = 'neutral', children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span
      style={{
        ...tones[tone],
        fontSize: 'var(--fs-caption)',
        fontWeight: 500,
        padding: '2px 8px',
        borderRadius: 'var(--radius-sm)',
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        lineHeight: 1.4,
      }}
    >{children}</span>
  );
}

export function toneForDeviceState(state: string): Tone {
  switch (state) {
    case 'online': return 'success';
    case 'monitoring_issue': return 'warning';
    case 'offline': return 'error';
    case 'maintenance': return 'muted';
    default: return 'neutral';
  }
}

export function toneForSeverity(sev: string): Tone {
  switch (sev) {
    case 'critical': return 'error';
    case 'warning': return 'warning';
    case 'info': return 'info';
    default: return 'neutral';
  }
}

export function toneForEnv(env: string): Tone {
  switch (env) {
    case 'dev': return 'info';
    case 'test': return 'warning';
    case 'uat': return 'muted';
    case 'prod': return 'error';
    default: return 'neutral';
  }
}
