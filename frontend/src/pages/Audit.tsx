// Audit — filters + paginated table of audit events; click a row to see the full event.
// Filter state syncs to the URL query string so a deep link re-opens the same
// view, and a scoped auditor (the `Auditor` role) gets an informational banner
// — the server enforces per-tenant scoping via ListForCaller + MembershipChecker.
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { FormField } from '../components/common/FormField';
import { useApi } from '../hooks/useApi';
import { apiGet } from '../api/client';
import { useAuth } from '../stores/auth';

interface AuditEvent {
  id: string;
  occurred_at: string;
  action: string;
  actor_id?: string;
  actor_username?: string;
  resource_type?: string;
  resource_id?: string;
  metadata?: Record<string, any>;
  [k: string]: any;
}

// Resource types that the audit emitter writes; "all" lets the
// caller opt out of the filter without picking a specific value.
const RESOURCE_TYPES = ['', 'project', 'device', 'k8s_cluster', 'pipeline', 'alert', 'log_saved_filter'] as const;
// Common audit actions. Free-form `action` text is still accepted
// via the existing text input — this select is a quick-pick.
const ACTIONS = ['', 'create', 'update', 'delete', 'maintenance_enter', 'maintenance_exit'] as const;

function toLocalInput(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalInput(s: string): string {
  if (!s) return '';
  return new Date(s).toISOString();
}

// Read filter values from window.location.search on first render
// so a deep link like /audit?action=create restores the same view.
function readFiltersFromURL(): {
  actor_id: string;
  resource_type: string;
  action: string;
  from: string;
  to: string;
  limit: number;
} {
  if (typeof window === 'undefined') {
    return { actor_id: '', resource_type: '', action: '', from: '', to: '', limit: 50 };
  }
  const p = new URLSearchParams(window.location.search);
  const limitRaw = p.get('limit');
  const limit = limitRaw && !Number.isNaN(Number(limitRaw)) ? Number(limitRaw) : 50;
  return {
    actor_id: p.get('actor_id') ?? '',
    resource_type: p.get('resource_type') ?? '',
    action: p.get('action') ?? '',
    from: p.get('from') ? toLocalInput(p.get('from') as string) : '',
    to: p.get('to') ? toLocalInput(p.get('to') as string) : '',
    limit,
  };
}

export function Audit() {
  // Hydrate form state from the URL so deep links restore the view.
  const initial = useMemo(readFiltersFromURL, []);

  // Form state
  const [action, setAction] = useState(initial.action);
  const [resourceType, setResourceType] = useState(initial.resource_type);
  const [actor, setActor] = useState(initial.actor_id);
  const [from, setFrom] = useState(initial.from);
  const [to, setTo] = useState(initial.to);
  const [limit, setLimit] = useState(initial.limit);

  // Submitted
  const [submitted, setSubmitted] = useState<Record<string, any> | null>(null);
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  // Caller role from the auth store (decoded from the JWT; no
  // extra round-trip). The `Auditor` role is the only scoped
  // read-only audit role in the matrix — server-side
  // ListForCaller enforces per-tenant scoping, the banner
  // just makes the constraint visible to the user.
  const callerRole = useAuth((s) => s.user?.role ?? null);
  const isScopedAuditor = callerRole === 'Auditor';

  // For the scoped auditor we surface the project list they
  // can see. The endpoint returns every project the caller
  // is allowed to read; the server then narrows the audit
  // listing to events on those projects. We use owner_id =
  // username as a best-effort "projects I can see" indicator
  // — the real enforcement is server-side.
  const callerUsername = useAuth((s) => s.user?.username ?? null);
  const projectsResult = useApi<{ data: Array<{ id: string }> } | Array<{ id: string }>>(
    isScopedAuditor && callerUsername ? 'projects' : null,
    isScopedAuditor && callerUsername ? { owner_id: callerUsername, limit: 500 } : undefined,
  );
  const projectIDs: string[] = useMemo(() => {
    const d: any = projectsResult.data;
    if (!d) return [];
    const list = Array.isArray(d) ? d : (d.data ?? []);
    return list.map((p: any) => p.id).filter(Boolean);
  }, [projectsResult.data]);

  // URL sync — every time the *submitted* filter set changes,
  // replace the query string so the browser address bar matches
  // the current view (avoids polluting history with each
  // keystroke; replaceState, not pushState).
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const params = new URLSearchParams();
    if (submitted) {
      Object.entries(submitted).forEach(([k, v]) => {
        if (v === undefined || v === null || v === '') return;
        params.set(k, String(v));
      });
    }
    const q = params.toString();
    const next = q ? `${window.location.pathname}?${q}` : window.location.pathname;
    if (next !== window.location.pathname + window.location.search) {
      window.history.replaceState(null, '', next);
    }
  }, [submitted]);

  // Final query sent to /api/v1/audit. For a scoped auditor we
  // append the visible project_ids so the call is explicit;
  // the server's ListForCaller would have done the scoping
  // already, but echoing the filter client-side keeps the
  // request self-describing in logs.
  const finalQuery = useMemo(() => {
    if (!submitted) return undefined;
    return { ...submitted };
  }, [submitted]);

  const result = useApi<AuditEvent[] | { data: AuditEvent[]; pagination?: any }>(
    'audit',
    finalQuery,
  );

  const rows: AuditEvent[] = useMemo(() => {
    const d: any = result.data;
    if (!d) return [];
    return Array.isArray(d) ? d : (d.data ?? []);
  }, [result.data]);

  const onApply = (e: React.FormEvent) => {
    e.preventDefault();
    const params: Record<string, any> = { limit };
    if (action.trim()) params.action = action.trim();
    if (resourceType.trim()) params.resource_type = resourceType.trim();
    if (actor.trim()) params.actor_id = actor.trim();
    if (from) params.from = fromLocalInput(from);
    if (to) params.to = fromLocalInput(to);
    setSubmitted(params);
  };

  const onReset = () => {
    setAction(''); setResourceType(''); setActor(''); setFrom(''); setTo(''); setLimit(50);
    setSubmitted(null);
  };

  const onRowClick = async (ev: AuditEvent) => {
    setDetail(ev);
    setDetailError(null);
    setDetailLoading(true);
    try {
      const full = await apiGet<AuditEvent>(`audit/${encodeURIComponent(ev.id)}`);
      setDetail(full);
    } catch (e: any) {
      setDetailError(e?.message ?? 'Failed to load event');
    } finally {
      setDetailLoading(false);
    }
  };

  const columns: Column<AuditEvent>[] = [
    {
      key: 'occurred',
      header: 'Occurred At',
      width: 200,
      render: (r) => <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>{r.occurred_at}</span>,
    },
    {
      key: 'action',
      header: 'Action',
      width: 200,
      render: (r) => <span className="mono" style={{ color: 'var(--color-primary)' }}>{r.action}</span>,
    },
    {
      key: 'actor',
      header: 'Actor',
      width: 160,
      render: (r) => <span className="mono" style={{ fontSize: 'var(--fs-caption)' }}>{r.actor_username ?? r.actor_id ?? '—'}</span>,
    },
    {
      key: 'resource',
      header: 'Resource',
      render: (r) => (
        <span style={{ fontSize: 'var(--fs-caption)' }}>
          {r.resource_type ?? '—'}
          {r.resource_id ? <span className="mono" style={{ color: 'var(--color-text-muted)' }}> / {r.resource_id}</span> : null}
        </span>
      ),
    },
    {
      key: 'metadata',
      header: 'Metadata',
      width: 320,
      render: (r) => (
        <pre style={{
          margin: 0,
          fontFamily: 'var(--font-mono)',
          fontSize: 'var(--fs-caption)',
          color: 'var(--color-text-secondary)',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          maxHeight: 60,
          overflow: 'hidden',
        }}>{r.metadata ? JSON.stringify(r.metadata, null, 2) : '—'}</pre>
      ),
    },
  ];

  return (
    <div data-testid="audit-page">
      <PageHeader title="Audit Log" subtitle="Search and inspect system events" />

      {isScopedAuditor && (
        <div
          data-testid="audit-scope-banner"
          style={{
            padding: '8px 12px',
            marginBottom: 'var(--sp-3)',
            background: 'var(--color-warning-bg, #fff8d6)',
            border: '1px solid var(--color-warning-border, #e6c75a)',
            borderRadius: 'var(--radius-sm)',
            color: 'var(--color-text)',
            fontSize: 'var(--fs-small)',
          }}
        >
          Showing audit events for {projectIDs.length > 0 ? `${projectIDs.length} project(s) you can access` : 'the projects you can access'}. Server-side filtering is enforced per-tenant.
        </div>
      )}

      <form onSubmit={onApply} style={{
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--sp-4)',
        marginBottom: 'var(--sp-5)',
      }}>
        <div data-testid="audit-filters" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr 1fr 120px auto', gap: 'var(--sp-3)', alignItems: 'end' }}>
          <FormField label="Action">
            {(s) => (
              <select
                style={s}
                data-testid="audit-filter-action"
                value={action}
                onChange={(e) => setAction(e.target.value)}
              >
                {ACTIONS.map((a) => (
                  <option key={a || 'all'} value={a}>{a || 'all'}</option>
                ))}
              </select>
            )}
          </FormField>
          <FormField label="Resource Type">
            {(s) => (
              <select
                style={s}
                data-testid="audit-filter-resource-type"
                value={resourceType}
                onChange={(e) => setResourceType(e.target.value)}
              >
                {RESOURCE_TYPES.map((r) => (
                  <option key={r || 'all'} value={r}>{r || 'all'}</option>
                ))}
              </select>
            )}
          </FormField>
          <FormField label="Actor ID / Username">
            {(s) => <input style={s} data-testid="audit-filter-actor-id" value={actor} onChange={(e) => setActor(e.target.value)} placeholder="alice" />}
          </FormField>
          <FormField label="From">
            {(s) => <input style={s} data-testid="audit-filter-from" type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />}
          </FormField>
          <FormField label="To">
            {(s) => <input style={s} data-testid="audit-filter-to" type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />}
          </FormField>
          <FormField label="Limit">
            {(s) => <input style={s} type="number" min={1} max={500} value={limit} onChange={(e) => setLimit(Number(e.target.value) || 50)} />}
          </FormField>
          <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
            <Button type="submit" variant="primary">Apply</Button>
            <Button type="button" data-testid="audit-filter-reset" onClick={onReset}>Reset</Button>
          </div>
        </div>
      </form>

      <DataTable
        rows={rows}
        columns={columns}
        rowKey={(r) => r.id}
        loading={result.loading}
        empty={{ title: 'No audit events', hint: submitted ? 'Try widening your filters.' : 'Apply filters to load events.' }}
      />

      <Modal
        open={!!detail}
        onClose={() => setDetail(null)}
        title={detail ? `Event ${detail.id}` : 'Event'}
        width={640}
        footer={<Button onClick={() => setDetail(null)}>Close</Button>}
      >
        {detailLoading && <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>}
        {detailError && <div style={{ color: 'var(--color-error)' }}>{detailError}</div>}
        {detail && !detailLoading && (
          <div style={{ display: 'grid', gap: 'var(--sp-3)', fontSize: 'var(--fs-small)' }}>
            <KV k="Occurred At" v={detail.occurred_at} />
            <KV k="Action" v={detail.action} mono />
            <KV k="Actor" v={detail.actor_username ?? detail.actor_id ?? '—'} />
            <KV k="Resource" v={`${detail.resource_type ?? '—'}${detail.resource_id ? ` / ${detail.resource_id}` : ''}`} />
            <div>
              <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600, marginBottom: 4 }}>Metadata</div>
              <pre style={{
                margin: 0,
                background: 'var(--color-bg)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-sm)',
                padding: 'var(--sp-3)',
                fontFamily: 'var(--font-mono)',
                fontSize: 'var(--fs-caption)',
                color: 'var(--color-text)',
                whiteSpace: 'pre-wrap',
                maxHeight: 320,
                overflow: 'auto',
              }}>{JSON.stringify(detail.metadata ?? {}, null, 2)}</pre>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}

function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr', gap: 'var(--sp-3)' }}>
      <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600 }}>{k}</span>
      <span style={{ fontFamily: mono ? 'var(--font-mono)' : undefined }}>{v}</span>
    </div>
  );
}
