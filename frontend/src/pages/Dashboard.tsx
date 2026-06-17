// Dashboard — landing page for authenticated users.
// Top: 4 stats cards (projects, devices, physical hosts, open alerts).
// Bottom: Recent Activity from /api/v1/audit?limit=10 rendered in a DataTable.

import { useTranslation } from 'react-i18next';
import { useApi } from '../hooks/useApi';
import { useAuth } from '../stores/auth';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge } from '../components/common/Badge';

interface CountResponse { data: unknown[]; pagination: { total: number } }
interface AuditEvent {
  id: string;
  actor: string;
  action: string;
  resource_type: string;
  resource_id: string;
  occurred_at: string;
}
interface AuditResponse { data: AuditEvent[]; pagination: { total: number } }

const gridStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
  gap: 'var(--sp-4)',
  marginBottom: 'var(--sp-7)',
};

const cardStyle: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-md)',
  padding: 'var(--sp-5)',
  display: 'flex',
  flexDirection: 'column',
  gap: 'var(--sp-2)',
};

const cardLabelStyle: React.CSSProperties = {
  fontSize: 'var(--fs-caption)',
  color: 'var(--color-text-secondary)',
  textTransform: 'uppercase',
  letterSpacing: 0.5,
  fontWeight: 600,
};

const cardValueStyle: React.CSSProperties = {
  fontFamily: 'var(--font-mono)',
  fontSize: '2.25rem',
  fontWeight: 500,
  color: 'var(--color-text)',
  fontVariantNumeric: 'tabular-nums',
  lineHeight: 1.1,
};

const cardSubtitleStyle: React.CSSProperties = {
  fontSize: 'var(--fs-caption)',
  color: 'var(--color-text-muted)',
};

const valueToneColor: Record<string, string> = {
  default: 'var(--color-text)',
  alert: 'var(--color-warning)',
};

function StatsCard({
  label,
  value,
  subtitle,
  tone = 'default',
  loading,
  locale,
}: {
  label: string;
  value: number | null;
  subtitle: string;
  tone?: 'default' | 'alert';
  loading?: boolean;
  locale: string;
}) {
  // Intl.NumberFormat picks locale-appropriate separators
  // and digit grouping (1,000 vs 1.000 vs 1 000). Falls back
  // to Intl default for any unsupported locale tag.
  const formatted = value == null
    ? null
    : new Intl.NumberFormat(locale).format(value);
  return (
    <div style={cardStyle}>
      <div style={cardLabelStyle}>{label}</div>
      <div style={{ ...cardValueStyle, color: valueToneColor[tone] }}>
        {loading || formatted == null ? '—' : formatted}
      </div>
      <div style={cardSubtitleStyle}>{subtitle}</div>
    </div>
  );
}

function formatTimestamp(s: string, locale: string): string {
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  // Intl.DateTimeFormat picks the locale's date format
  // (YYYY-MM-DD in zh-CN, M/D/YYYY in en-US, DD/MM/YYYY in
  // most of Europe). Falls back to the default formatter for
  // any locale tag Intl doesn't recognize.
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(d);
}

function actionTone(action: string) {
  if (action === 'create' || action === 'grant') return 'success' as const;
  if (action === 'delete' || action === 'revoke') return 'error' as const;
  if (action === 'update' || action === 'modify') return 'info' as const;
  return 'neutral' as const;
}

export function Dashboard() {
  const { t, i18n } = useTranslation('dashboard');
  const user = useAuth((s) => s.user);

  // Stats — useApi aborts on unmount, so all four run in parallel.
  const projects = useApi<CountResponse>('projects', { limit: 1 });
  const devices = useApi<CountResponse>('devices', { limit: 1 });
  const hosts = useApi<CountResponse>('physical-hosts', { limit: 1 });
  const alerts = useApi<CountResponse>('alerts', { status: 'open', limit: 1 });
  const audit = useApi<AuditResponse>('audit', { limit: 10 });

  const cols: Column<AuditEvent>[] = [
    { key: 'actor', header: t('table.column.actor'), render: (r) => <span className="mono">{r.actor}</span> },
    { key: 'action', header: t('table.column.action'), render: (r) => <Badge tone={actionTone(r.action)}>{r.action}</Badge> },
    { key: 'resource_type', header: t('table.column.resource-type'), render: (r) => r.resource_type },
    {
      key: 'resource_id',
      header: t('table.column.resource-id'),
      render: (r) => <span className="mono" style={{ color: 'var(--color-text-muted)' }}>{r.resource_id}</span>,
    },
    {
      key: 'occurred_at',
      header: t('table.column.occurred-at'),
      align: 'right',
      render: (r) => (
        <span style={{ color: 'var(--color-text-muted)' }}>{formatTimestamp(r.occurred_at, i18n.language)}</span>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title={t('title')}
        subtitle={user ? t('welcome', { name: user.username }) : t('welcome-anonymous')}
      />

      <div style={gridStyle}>
        <StatsCard
          label={t('stats.projects')}
          value={projects.data?.pagination.total ?? null}
          subtitle={t('stats.projects-subtitle')}
          loading={projects.loading}
          locale={i18n.language}
        />
        <StatsCard
          label={t('stats.devices')}
          value={devices.data?.pagination.total ?? null}
          subtitle={t('stats.devices-subtitle')}
          loading={devices.loading}
          locale={i18n.language}
        />
        <StatsCard
          label={t('stats.physical-hosts')}
          value={hosts.data?.pagination.total ?? null}
          subtitle={t('stats.physical-hosts-subtitle')}
          loading={hosts.loading}
          locale={i18n.language}
        />
        <StatsCard
          label={t('stats.open-alerts')}
          value={alerts.data?.pagination.total ?? null}
          subtitle={t('stats.open-alerts-subtitle')}
          tone="alert"
          loading={alerts.loading}
          locale={i18n.language}
        />
      </div>

      <h2
        style={{
          fontSize: 'var(--fs-h3)',
          color: 'var(--color-text)',
          marginBottom: 'var(--sp-3)',
        }}
      >
        {t('recent.title')}
      </h2>
      <DataTable<AuditEvent>
        rows={audit.data?.data ?? null}
        columns={cols}
        rowKey={(r) => r.id}
        loading={audit.loading}
        empty={{
          title: t('recent.empty'),
          hint: t('recent.empty-hint'),
        }}
      />
    </div>
  );
}
