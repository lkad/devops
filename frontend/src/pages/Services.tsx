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
import { apiPost, apiDelete, apiGet } from '../api/client';
import { formatApiError } from '../api/errors';
import { Badge } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { PageHeader } from '../components/common/PageHeader';
import { EmptyState } from '../components/common/EmptyState';
import { DataTable, Column } from '../components/common/DataTable';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';
import { useAuth } from '../stores/auth';
import { OnCallEditor } from '../components/service-catalog/OnCallEditor';
import { RunbookEditor } from '../components/service-catalog/RunbookEditor';

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
  // P2.2 + P2.3 — embedded by GET /api/v1/services/:id.
  // Oncall is the current shift (null = no one on call);
  // runbook is a (possibly empty) array of entries
  // newest-first.
  oncall?: OnCall | null;
  runbook?: RunbookEntry[];
}

interface OnCall {
  id: string;
  service_id?: string;
  user: string;
  shift_start: string;
  shift_end: string;
}

interface RunbookEntry {
  id: string;
  service_id: string;
  title: string;
  body: string;
  created_at: string;
  updated_at: string;
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
  // detailTick bumps whenever the detail's embedded
  // oncall + runbook need a fresh fetch (after a
  // write). Passed down to ServiceDetail so it can
  // re-key its useApi on the GET /services/:id call.
  const [detailTick, setDetailTick] = useState(0);
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
              onChanged={() => {
                // Re-fetch the list so the embedded
                // oncall + runbook in the cached GET
                // response is current. reload() refires
                // the list query; we additionally
                // re-fetch the single service so the
                // detail's useApi cache is in sync.
                reload();
                setDetailTick((n) => n + 1);
              }}
              tick={detailTick}
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
  onChanged,
  tick,
}: {
  svc: Service;
  onDeleted: () => void;
  onChanged: () => void;
  tick: number;
}) {
  const { data: health, loading, error } = useApi<HealthResult>(
    `services/${svc.id}/health`,
  );
  // Re-fetch the single service so the embedded
  // oncall + runbook are current after a write.
  // The tick prop bumps on each write; append it to
  // the path so useApi sees a "new" URL and refires.
  const { data: detail, reload: reloadDetail } = useApi<Service>(
    `services/${svc.id}?tick=${tick}`,
  );
  const live: Service = detail ?? svc;
  const { push: toast } = useToast();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  // Oncall + runbook modals. Split state so the two
  // forms can be open independently — the modal for
  // adding an on-call shift and the one for adding a
  // runbook entry are distinct, so a single
  // `creating` would be a footgun.
  const [addingOnCall, setAddingOnCall] = useState(false);
  const [addingRunbook, setAddingRunbook] = useState(false);
  // Explicit-text Editor modals (separate state so
  // the inline AddOnCallModal/AddRunbookModal and the
  // new OnCallEditor/RunbookEditor components don't
  // collide if both are open simultaneously). Gated
  // to Operator/SuperAdmin via the role check below.
  const [editorOnCallOpen, setEditorOnCallOpen] = useState(false);
  const [editorRunbookOpen, setEditorRunbookOpen] = useState(false);
  const [confirmDeleteShift, setConfirmDeleteShift] = useState<string | null>(null);
  const [confirmDeleteEntry, setConfirmDeleteEntry] = useState<string | null>(null);
  // D 子项目 — only Operator/SuperAdmin see the
  // explicit-text "Add on-call" / "Add runbook"
  // buttons. Other roles still see the legacy +
  // icon button so they can keep creating entries
  // through the inline modal if their role permits
  // server-side.
  const callerRole = useAuth((s) => s.user?.role ?? null);
  const canAdd = callerRole === 'Operator' || callerRole === 'SuperAdmin';

  async function handleDelete() {
    setDeleting(true);
    try {
      await apiDelete(`services/${svc.id}`);
      onDeleted();
    } catch (e: any) {
      toast(formatApiError('Delete failed', e), 'error');
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

      <OnCallBlock
        oncall={live.oncall ?? null}
        onAdd={() => setAddingOnCall(true)}
        onDelete={async (shiftId) => {
          setConfirmDeleteShift(shiftId);
        }}
      />
      {canAdd && (
        <div style={{ marginTop: 'var(--sp-2)' }}>
          <Button
            variant="secondary"
            data-testid="service-add-oncall"
            onClick={() => setEditorOnCallOpen(true)}
          >
            Add on-call
          </Button>
        </div>
      )}

      <h3 style={{ fontSize: 'var(--fs-h3)', marginBottom: 'var(--sp-3)', marginTop: 'var(--sp-5)' }}>
        Runbook
      </h3>
      <RunbookBlock
        entries={live.runbook ?? []}
        onAdd={() => setAddingRunbook(true)}
        onDelete={async (entryId) => {
          setConfirmDeleteEntry(entryId);
        }}
      />
      {canAdd && (
        <div style={{ marginTop: 'var(--sp-2)' }}>
          <Button
            variant="secondary"
            data-testid="service-add-runbook"
            onClick={() => setEditorRunbookOpen(true)}
          >
            Add runbook
          </Button>
        </div>
      )}

      <h3 style={{ fontSize: 'var(--fs-h3)', marginBottom: 'var(--sp-3)', marginTop: 'var(--sp-5)' }}>
        Recent deploys
      </h3>
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

      {addingOnCall && (
        <AddOnCallModal
          serviceId={svc.id}
          onClose={() => setAddingOnCall(false)}
          onCreated={() => {
            setAddingOnCall(false);
            toast('On-call shift added', 'success');
            onChanged();
          }}
        />
      )}
      {addingRunbook && (
        <AddRunbookModal
          serviceId={svc.id}
          onClose={() => setAddingRunbook(false)}
          onCreated={() => {
            setAddingRunbook(false);
            toast('Runbook entry added', 'success');
            onChanged();
          }}
        />
      )}
      <OnCallEditor
        open={editorOnCallOpen}
        onClose={() => setEditorOnCallOpen(false)}
        serviceID={svc.id}
        onSaved={() => {
          setEditorOnCallOpen(false);
          toast('On-call shift added', 'success');
          onChanged();
        }}
      />
      <RunbookEditor
        open={editorRunbookOpen}
        onClose={() => setEditorRunbookOpen(false)}
        serviceID={svc.id}
        onSaved={() => {
          setEditorRunbookOpen(false);
          toast('Runbook entry added', 'success');
          onChanged();
        }}
      />
      {confirmDeleteShift && (
        <ConfirmDeleteModal
          title="Delete on-call shift?"
          message="The shift will be removed from the rotation. Anyone currently on call will lose their shift immediately."
          onCancel={() => setConfirmDeleteShift(null)}
          onConfirm={async () => {
            const id = confirmDeleteShift;
            setConfirmDeleteShift(null);
            try {
              await apiDelete(`services/${svc.id}/oncall/${id}`);
              toast('On-call shift removed', 'success');
              onChanged();
            } catch (e: any) {
              toast(formatApiError('Delete shift failed', e), 'error');
            }
          }}
        />
      )}
      {confirmDeleteEntry && (
        <ConfirmDeleteModal
          title="Delete runbook entry?"
          message="The runbook entry will be removed permanently."
          onCancel={() => setConfirmDeleteEntry(null)}
          onConfirm={async () => {
            const id = confirmDeleteEntry;
            setConfirmDeleteEntry(null);
            try {
              await apiDelete(`services/${svc.id}/runbook/${id}`);
              toast('Runbook entry removed', 'success');
              onChanged();
            } catch (e: any) {
              toast(formatApiError('Delete entry failed', e), 'error');
            }
          }}
        />
      )}
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

function OnCallBlock({
  oncall,
  onAdd,
  onDelete,
}: {
  oncall: OnCall | null;
  onAdd: () => void;
  onDelete: (shiftId: string) => void;
}) {
  return (
    <div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 'var(--sp-2)',
        }}
      >
        <h3 style={{ fontSize: 'var(--fs-h3)' }}>On-call</h3>
        <Button variant="secondary" onClick={onAdd}>
          +
        </Button>
      </div>
      {!oncall ? (
        <div
          style={{
            padding: 'var(--sp-3) var(--sp-4)',
            background: 'var(--color-surface-elevated)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-md)',
            color: 'var(--color-text-muted)',
            fontSize: 'var(--fs-small)',
          }}
        >
          No one is on call for this service right now.
        </div>
      ) : (
        <div
          style={{
            padding: 'var(--sp-3) var(--sp-4)',
            background: 'var(--color-surface-elevated)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-md)',
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--sp-3)',
          }}
        >
          <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
            Currently on call:
          </span>
          <span className="mono" style={{ fontWeight: 600 }}>
            {oncall.user}
          </span>
          <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>
            until {relative(oncall.shift_end)}
          </span>
          {/* The × deletes the currently-displayed shift
              (which is the only shift the block has
              state about). Future iterations may want
              a "rotation timeline" view that lists
              upcoming shifts; for now the wire response
              only carries the current one. */}
          <span style={{ flex: 1 }} />
          <Button
            variant="secondary"
            onClick={() => onDelete(oncall.id)}
          >
            ×
          </Button>
        </div>
      )}
    </div>
  );
}

function RunbookBlock({
  entries,
  onAdd,
  onDelete,
}: {
  entries: RunbookEntry[];
  onAdd: () => void;
  onDelete: (entryId: string) => void;
}) {
  return (
    <div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 'var(--sp-2)',
        }}
      >
        <div /> {/* placeholder so the h3 above aligns with the + button on the right */}
        <Button variant="secondary" onClick={onAdd}>
          +
        </Button>
      </div>
      {entries.length === 0 ? (
        <div
          style={{
            padding: 'var(--sp-3) var(--sp-4)',
            border: '1px dashed var(--color-border)',
            borderRadius: 'var(--radius-md)',
            color: 'var(--color-text-muted)',
            fontSize: 'var(--fs-small)',
          }}
        >
          No runbook entries yet. Add a procedure the on-call
          should follow when this service is on fire.
        </div>
      ) : (
        <ol style={{ listStyle: 'none', margin: 0, padding: 0 }}>
          {entries.map((e) => (
            <li
              key={e.id}
              style={{
                background: 'var(--color-surface-elevated)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-sm)',
                padding: 'var(--sp-3) var(--sp-4)',
                marginBottom: 'var(--sp-2)',
                position: 'relative',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 'var(--sp-2)' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ fontWeight: 600, marginBottom: 'var(--sp-1)' }}>{e.title}</div>
                  <pre
                    style={{
                      margin: 0,
                      whiteSpace: 'pre-wrap',
                      fontFamily: 'inherit',
                      fontSize: 'var(--fs-small)',
                      color: 'var(--color-text-secondary)',
                    }}
                  >
                    {e.body}
                  </pre>
                </div>
                <Button
                  variant="secondary"
                  onClick={() => onDelete(e.id)}
                >
                  ×
                </Button>
              </div>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

// AddOnCallModal — POST a new on-call shift.
//
// Date inputs use `datetime-local`, which the browser
// renders as the operator's local timezone. The
// toISOString conversion below treats the input as
// local time and emits an RFC3339 UTC string for the
// backend. If the field is blank the conversion skips
// the field entirely (the server-side validator
// returns 400).
function AddOnCallModal({
  serviceId,
  onClose,
  onCreated,
}: {
  serviceId: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [userEmail, setUserEmail] = useState('');
  const [shiftStart, setShiftStart] = useState('');
  const [shiftEnd, setShiftEnd] = useState('');
  const [scope, setScope] = useState<'service' | 'global'>('service');
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const { push: toast } = useToast();

  // Convert a "datetime-local" string (e.g. "2026-06-11T08:00")
  // to an RFC3339 UTC string. The browser's
  // datetime-local input does not carry a timezone, so
  // the value is interpreted as the operator's local
  // time — which is what the operator expects.
  function toRFC3339(local: string): string {
    if (!local) return '';
    const d = new Date(local);
    if (Number.isNaN(d.getTime())) return local;
    return d.toISOString();
  }

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost(`services/${serviceId}/oncall`, {
        user_email: userEmail,
        shift_start: toRFC3339(shiftStart),
        shift_end: toRFC3339(shiftEnd),
        scope,
      });
      onCreated();
    } catch (e: any) {
      setErr(formatApiError('Add on-call failed', e));
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Add on-call shift"
      width={520}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            onClick={submit}
            disabled={submitting || !userEmail || !shiftStart || !shiftEnd}
          >
            {submitting ? 'Adding…' : 'Add shift'}
          </Button>
        </>
      }
    >
      {err && (
        <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>
          {err}
        </div>
      )}
      <FormField label="User email">
        {(s) => (
          <input
            style={s}
            type="email"
            value={userEmail}
            onChange={(e) => setUserEmail(e.target.value)}
            placeholder="alice@example.com"
          />
        )}
      </FormField>
      <FormField label="Shift start">
        {(s) => (
          <input
            style={s}
            type="datetime-local"
            value={shiftStart}
            onChange={(e) => setShiftStart(e.target.value)}
          />
        )}
      </FormField>
      <FormField label="Shift end">
        {(s) => (
          <input
            style={s}
            type="datetime-local"
            value={shiftEnd}
            onChange={(e) => setShiftEnd(e.target.value)}
          />
        )}
      </FormField>
      <FormField label="Scope">
        {(s) => (
          <div style={{ display: 'flex', gap: 'var(--sp-3)', alignItems: 'center' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 'var(--fs-small)' }}>
              <input
                type="radio"
                name="oncall-scope"
                value="service"
                checked={scope === 'service'}
                onChange={() => setScope('service')}
              />
              Per-service
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 'var(--fs-small)' }}>
              <input
                type="radio"
                name="oncall-scope"
                value="global"
                checked={scope === 'global'}
                onChange={() => setScope('global')}
              />
              Global fallback
            </label>
          </div>
        )}
      </FormField>
    </Modal>
  );
}

// AddRunbookModal — POST a new runbook entry.
function AddRunbookModal({
  serviceId,
  onClose,
  onCreated,
}: {
  serviceId: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const { push: toast } = useToast();

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost(`services/${serviceId}/runbook`, { title, body });
      onCreated();
    } catch (e: any) {
      setErr(formatApiError('Add runbook entry failed', e));
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Add runbook entry"
      width={520}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={submitting || !title}>
            {submitting ? 'Adding…' : 'Add entry'}
          </Button>
        </>
      }
    >
      {err && (
        <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>
          {err}
        </div>
      )}
      <FormField label="Title">
        {(s) => (
          <input
            style={s}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Restart procedure"
          />
        )}
      </FormField>
      <FormField label="Body">
        {(s) => (
          <textarea
            style={{ ...s, minHeight: 120, fontFamily: 'inherit' }}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder={'1. ssh in\n2. systemctl restart'}
          />
        )}
      </FormField>
    </Modal>
  );
}

// ConfirmDeleteModal — a tiny generic confirm
// dialog. Reused by the on-call + runbook delete
// buttons. The footer uses the danger variant; the
// caller's onConfirm is the action.
function ConfirmDeleteModal({
  title,
  message,
  onCancel,
  onConfirm,
}: {
  title: string;
  message: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <Modal
      open
      onClose={onCancel}
      title={title}
      width={420}
      footer={
        <>
          <Button variant="secondary" onClick={onCancel}>Cancel</Button>
          <Button variant="danger" onClick={onConfirm}>Delete</Button>
        </>
      }
    >
      <div style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--fs-small)' }}>
        {message}
      </div>
    </Modal>
  );
}
