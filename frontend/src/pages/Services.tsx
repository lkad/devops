// Services — microservice catalog page.
//
// Two-pane layout:
//   Left: service list (name, tier badge, owner, health pill)
//   Right: selected service's detail (info + recent deploys)
//
// The page exists so an on-call operator can answer
// "what's unhealthy right now, and which deploy last
// touched it" in one screen instead of tree-walking
// PhysicalHosts / Pipelines / Logs.
//
// See docs/design/service-layer.md and
// openspec/specs/service-catalog/spec.md for the design.

import { useEffect, useMemo, useState } from 'react';
import { useApi } from '../hooks/useApi';
import { apiPost, apiDelete } from '../api/client';
import { Badge } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { PageHeader } from '../components/common/PageHeader';
import { EmptyState } from '../components/common/EmptyState';
import { DataTable, Column } from '../components/common/DataTable';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';

type Tier = 'critical' | 'important' | 'standard' | '' | string;
type HealthStatus = 'healthy' | 'degraded' | 'unknown';

interface Service {
  id: string;
  name: string;
  description?: string;
  owner?: string;
  repository_url?: string;
  tier: Tier;
  created_at?: string;
  updated_at?: string;
}

interface HealthResult {
  service_id: string;
  status: HealthStatus;
  derived_from: string;
  reason?: string;
  last_run?: HealthRun;
  recent_runs: HealthRun[];
}

interface HealthRun {
  id: string;
  status: string;
  started_at: string;
  duration_ms: number;
}

function tierTone(t: Tier): 'info' | 'success' | 'neutral' {
  if (t === 'critical') return 'info';
  if (t === 'important') return 'success';
  return 'neutral';
}

function tierLabel(t: Tier): string {
  if (t === 'critical') return 'Critical';
  if (t === 'important') return 'Important';
  if (t === 'standard') return 'Standard';
  return t || '—';
}

function healthTone(s: HealthStatus): 'success' | 'error' | 'warning' | 'neutral' {
  if (s === 'healthy') return 'success';
  if (s === 'degraded') return 'error';
  return 'warning';
}

function healthLabel(s: HealthStatus): string {
  if (s === 'healthy') return 'Healthy';
  if (s === 'degraded') return 'Degraded';
  return 'Unknown';
}

function fmtDate(s?: string) {
  if (!s) return '—';
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}

function relative(s?: string): string {
  if (!s) return '—';
  const t = new Date(s).getTime();
  if (Number.isNaN(t)) return s;
  const delta = Date.now() - t;
  if (delta < 60_000) return `${Math.floor(delta / 1000)}s ago`;
  if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`;
  if (delta < 86_400_000) return `${Math.floor(delta / 3_600_000)}h ago`;
  return `${Math.floor(delta / 86_400_000)}d ago`;
}

function fmtDuration(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}

export function Services() {
  const { data, loading, error, reload } = useApi<{ data: Service[] }>('services');
  const services: Service[] = useMemo(() => data?.data ?? [], [data]);
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const { push: toast } = useToast();

  // Auto-select first service.
  useEffect(() => {
    if (!selectedId && services.length > 0) setSelectedId(services[0].id);
  }, [selectedId, services]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return services;
    return services.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        (s.owner ?? '').toLowerCase().includes(q),
    );
  }, [services, query]);

  const selected = services.find((s) => s.id === selectedId) ?? null;

  return (
    <div>
      <PageHeader
        title="Services"
        subtitle="Microservice catalog and health rollup"
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            + New Service
          </Button>
        }
      />

      {error && <div style={{ color: 'var(--color-error)' }}>{error}</div>}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '380px 1fr',
          gap: 'var(--sp-4)',
          minHeight: 400,
        }}
      >
        {/* Left: list with search */}
        <div
          style={{
            background: 'var(--color-surface)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-md)',
            overflow: 'hidden',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div style={{ padding: 'var(--sp-3) var(--sp-4)', borderBottom: '1px solid var(--color-border-subtle)' }}>
            <input
              type="text"
              placeholder="Search by name or owner…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{
                width: '100%',
                padding: 'var(--sp-2) var(--sp-3)',
                background: 'var(--color-surface-elevated)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--color-text)',
                fontSize: 'var(--fs-small)',
              }}
            />
          </div>
          <div style={{ overflow: 'auto', flex: 1 }}>
            {loading ? (
              <div style={{ padding: 'var(--sp-5)', color: 'var(--color-text-muted)' }}>Loading…</div>
            ) : filtered.length === 0 ? (
              <div style={{ padding: 'var(--sp-4)' }}>
                <EmptyState
                  title={services.length === 0 ? 'No services yet' : 'No matches'}
                  hint={
                    services.length === 0
                      ? 'Create one with the + New Service button.'
                      : 'Try a different search term.'
                  }
                />
              </div>
            ) : (
              <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
                {filtered.map((s) => (
                  <ServiceRow
                    key={s.id}
                    svc={s}
                    active={s.id === selectedId}
                    onClick={() => setSelectedId(s.id)}
                  />
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* Right: detail */}
        <div>
          {selected ? (
            <ServiceDetail
              svc={selected}
              onDeleted={() => {
                toast(`Service ${selected.name} deleted`, 'success');
                setSelectedId(null);
                reload();
              }}
            />
          ) : (
            <EmptyState
              title="Select a service"
              hint="Pick one from the left to view its health and recent deploys."
            />
          )}
        </div>
      </div>

      {creating && (
        <NewServiceModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            toast('Service created', 'success');
            reload();
          }}
        />
      )}
    </div>
  );
}

function ServiceRow({
  svc,
  active,
  onClick,
}: {
  svc: Service;
  active: boolean;
  onClick: () => void;
}) {
  // Health is fetched separately per row to keep the list
  // view responsive even if a health query is slow. The
  // request fires on every render — cheap on the server
  // side and cached in the browser by the api client.
  const { data: health } = useApi<HealthResult>(`services/${svc.id}/health`);
  return (
    <li
      onClick={onClick}
      style={{
        padding: 'var(--sp-3) var(--sp-4)',
        cursor: 'pointer',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: active ? 'var(--color-primary-muted)' : 'transparent',
        borderLeft: active
          ? '2px solid var(--color-primary)'
          : '2px solid transparent',
      }}
    >
      <div className="mono" style={{ fontSize: 'var(--fs-mono)', marginBottom: 4 }}>
        {svc.name}
      </div>
      <div
        style={{
          display: 'flex',
          gap: 'var(--sp-2)',
          alignItems: 'center',
          fontSize: 'var(--fs-caption)',
          color: 'var(--color-text-secondary)',
          flexWrap: 'wrap',
        }}
      >
        <Badge tone={tierTone(svc.tier)}>{tierLabel(svc.tier)}</Badge>
        {health && (
          <Badge tone={healthTone(health.status)}>{healthLabel(health.status)}</Badge>
        )}
        {svc.owner && (
          <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
            {svc.owner}
          </span>
        )}
      </div>
    </li>
  );
}

function ServiceDetail({
  svc,
  onDeleted,
}: {
  svc: Service;
  onDeleted: () => void;
}) {
  const { data: health, loading, error } = useApi<HealthResult>(
    `services/${svc.id}/health`,
  );
  const { push: toast } = useToast();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);
    try {
      await apiDelete(`services/${svc.id}`);
      onDeleted();
    } catch (e: any) {
      toast(`Delete failed: ${e?.message ?? 'unknown'}`, 'error');
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  return (
    <div
      style={{
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--sp-5)',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          marginBottom: 'var(--sp-4)',
          gap: 'var(--sp-3)',
        }}
      >
        <div>
          <h2 className="mono" style={{ fontSize: 'var(--fs-h2)', marginBottom: 'var(--sp-1)' }}>
            {svc.name}
          </h2>
          {svc.description && (
            <div style={{ color: 'var(--color-text-secondary)' }}>{svc.description}</div>
          )}
        </div>
        <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
          <Badge tone={tierTone(svc.tier)}>{tierLabel(svc.tier)}</Badge>
          {health && (
            <Badge tone={healthTone(health.status)}>{healthLabel(health.status)}</Badge>
          )}
        </div>
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '120px 1fr',
          gap: 'var(--sp-2) var(--sp-4)',
          marginBottom: 'var(--sp-5)',
          fontSize: 'var(--fs-small)',
        }}
      >
        <div style={{ color: 'var(--color-text-secondary)' }}>Owner</div>
        <div>{svc.owner || '—'}</div>
        <div style={{ color: 'var(--color-text-secondary)' }}>Repository</div>
        <div>
          {svc.repository_url ? (
            <a href={svc.repository_url} target="_blank" rel="noreferrer">
              {svc.repository_url}
            </a>
          ) : (
            '—'
          )}
        </div>
        <div style={{ color: 'var(--color-text-secondary)' }}>Created</div>
        <div>{fmtDate(svc.created_at)}</div>
        {health?.last_run && (
          <>
            <div style={{ color: 'var(--color-text-secondary)' }}>Last deploy</div>
            <div>
              {health.last_run.id.slice(0, 8)} ·{' '}
              <span className="mono" style={{ fontSize: 'var(--fs-caption)' }}>
                {health.last_run.status}
              </span>{' '}
              · {relative(health.last_run.started_at)}
            </div>
          </>
        )}
        {health?.reason && (
          <>
            <div style={{ color: 'var(--color-text-secondary)' }}>Health reason</div>
            <div style={{ color: 'var(--color-text-muted)' }}>{health.reason}</div>
          </>
        )}
      </div>

      <h3 style={{ fontSize: 'var(--fs-h3)', marginBottom: 'var(--sp-3)' }}>Recent deploys</h3>
      {loading && <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>}
      {error && <div style={{ color: 'var(--color-error)' }}>{error}</div>}
      {health && (
        <RecentDeploys runs={health.recent_runs} />
      )}

      <div
        style={{
          marginTop: 'var(--sp-5)',
          paddingTop: 'var(--sp-4)',
          borderTop: '1px solid var(--color-border-subtle)',
          display: 'flex',
          justifyContent: 'flex-end',
        }}
      >
        {!confirmDelete ? (
          <Button variant="danger" onClick={() => setConfirmDelete(true)}>
            Delete service
          </Button>
        ) : (
          <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
            <Button variant="secondary" onClick={() => setConfirmDelete(false)} disabled={deleting}>
              Cancel
            </Button>
            <Button variant="danger" onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Deleting…' : 'Confirm delete'}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

function RecentDeploys({ runs }: { runs: HealthRun[] }) {
  if (runs.length === 0) {
    return (
      <div
        style={{
          padding: 'var(--sp-4)',
          color: 'var(--color-text-muted)',
          fontSize: 'var(--fs-small)',
          border: '1px dashed var(--color-border)',
          borderRadius: 'var(--radius-sm)',
        }}
      >
        No deploys yet for this service. A pipeline with this service as its
        service_id will appear here once it runs.
      </div>
    );
  }
  const columns: Column<HealthRun>[] = [
    {
      key: 'id',
      header: 'Run',
      render: (r) => <span className="mono">{r.id.slice(0, 8)}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (r) => <Badge tone={runTone(r.status)}>{r.status}</Badge>,
    },
    {
      key: 'started',
      header: 'Started',
      render: (r) => relative(r.started_at),
    },
    {
      key: 'duration',
      header: 'Duration',
      render: (r) => fmtDuration(r.duration_ms),
    },
  ];
  return (
    <DataTable
      rows={runs}
      columns={columns}
      rowKey={(r) => r.id}
      empty={{
        title: 'No runs yet',
        hint: 'A pipeline deploying this service has not been triggered.',
      }}
    />
  );
}

function runTone(s: string) {
  if (s === 'succeeded') return 'success' as const;
  if (s === 'failed' || s === 'error') return 'error' as const;
  if (s === 'running') return 'info' as const;
  if (s === 'cancelled') return 'muted' as const;
  if (s === 'pending') return 'warning' as const;
  return 'neutral' as const;
}

function NewServiceModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState('');
  const [tier, setTier] = useState<Tier>('standard');
  const [owner, setOwner] = useState('');
  const [description, setDescription] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost('services', {
        name,
        tier,
        owner,
        description,
        repository_url: repoUrl,
      });
      onCreated();
    } catch (e: any) {
      setErr(e?.message ?? 'create failed');
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="New Service"
      width={520}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={submitting || !name}>
            {submitting ? 'Creating…' : 'Create'}
          </Button>
        </>
      }
    >
      {err && <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{err}</div>}
      <FormField label="Name">
        {(s) => <input style={s} value={name} onChange={(e) => setName(e.target.value)} placeholder="payments-api" />}
      </FormField>
      <FormField label="Tier">
        {(s) => (
          <select style={s} value={tier} onChange={(e) => setTier(e.target.value as Tier)}>
            <option value="critical">Critical</option>
            <option value="important">Important</option>
            <option value="standard">Standard</option>
          </select>
        )}
      </FormField>
      <FormField label="Owner">
        {(s) => <input style={s} value={owner} onChange={(e) => setOwner(e.target.value)} placeholder="team-payments@example.com" />}
      </FormField>
      <FormField label="Repository URL">
        {(s) => <input style={s} value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} placeholder="https://git.example.com/payments-api" />}
      </FormField>
      <FormField label="Description">
        {(s) => <input style={s} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />}
      </FormField>
    </Modal>
  );
}
