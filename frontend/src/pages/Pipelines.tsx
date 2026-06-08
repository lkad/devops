// Pipelines — two-pane page.
// Left: pipeline list (name, target_type, last run status).
// Right: selected pipeline's run history (DataTable with Run ID, Status,
// Started, Duration, Triggered By, Actions).
// Top-right: "+ New Pipeline" + "Trigger" buttons.
// Trigger calls POST /pipelines/:id/trigger and shows a Toast.
// Clicking a Run row opens a modal with per-step status.

import { useEffect, useState } from 'react';
import { useApi } from '../hooks/useApi';
import { apiPost } from '../api/client';
import { Badge } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { PageHeader } from '../components/common/PageHeader';
import { EmptyState } from '../components/common/EmptyState';
import { DataTable, Column } from '../components/common/DataTable';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';

type RunStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'success' | 'error' | string;

interface Pipeline {
  id: string;
  name: string;
  target_type?: string;
  description?: string;
  last_run_status?: RunStatus;
  last_run_id?: string;
  created_at?: string;
}

interface Run {
  id: string;
  status: RunStatus;
  started_at?: string;
  finished_at?: string;
  duration_ms?: number;
  triggered_by?: string;
  pipeline_id?: string;
  steps?: RunStep[];
}

interface RunStep {
  name: string;
  status: RunStatus;
  duration_ms?: number;
  started_at?: string;
  finished_at?: string;
  log?: string;
}

function runTone(s: RunStatus) {
  if (s === 'succeeded' || s === 'success') return 'success' as const;
  if (s === 'failed' || s === 'error') return 'error' as const;
  if (s === 'running') return 'info' as const;
  if (s === 'cancelled') return 'muted' as const;
  if (s === 'pending') return 'warning' as const;
  return 'neutral' as const;
}

function fmtDuration(ms?: number) {
  if (ms === undefined || ms === null) return '—';
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  return `${m}m ${rs}s`;
}

function fmtDate(s?: string) {
  if (!s) return '—';
  try {
    const d = new Date(s);
    return d.toLocaleString();
  } catch {
    return s;
  }
}

export function Pipelines() {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [trigging, setTrigging] = useState(false);
  const [viewingRun, setViewingRun] = useState<string | null>(null);
  const { push: toast } = useToast();

  const { data, loading, error, reload } = useApi<{ data: Pipeline[] }>('pipelines');
  const pipelines: Pipeline[] = data?.data ?? [];

  // Auto-select the first pipeline.
  useEffect(() => {
    if (!selectedId && pipelines.length > 0) setSelectedId(pipelines[0].id);
  }, [selectedId, pipelines]);

  const selected = pipelines.find((p) => p.id === selectedId) ?? null;

  async function handleTrigger() {
    if (!selected) return;
    setTrigging(true);
    try {
      const r = await apiPost<Run>(`pipelines/${selected.id}/trigger`);
      toast(`Triggered run ${r.id}`, 'success');
      reload();
    } catch (e: any) {
      toast(`Trigger failed: ${e?.message ?? 'unknown'}`, 'error');
    } finally {
      setTrigging(false);
    }
  }

  return (
    <div>
      <PageHeader
        title="Pipelines"
        subtitle="CI/CD pipelines and run history"
        actions={
          <>
            <Button variant="primary" onClick={() => setCreating(true)}>
              + New Pipeline
            </Button>
            <Button
              variant="secondary"
              onClick={handleTrigger}
              disabled={!selected || trigging}
            >
              {trigging ? 'Triggering…' : 'Trigger'}
            </Button>
          </>
        }
      />

      {error && <div style={{ color: 'var(--color-error)' }}>{error}</div>}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '320px 1fr',
          gap: 'var(--sp-4)',
          minHeight: 400,
        }}
      >
        {/* Left pane: pipeline list */}
        <div
          style={{
            background: 'var(--color-surface)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-md)',
            overflow: 'auto',
          }}
        >
          {loading ? (
            <div style={{ padding: 'var(--sp-5)', color: 'var(--color-text-muted)' }}>Loading…</div>
          ) : pipelines.length === 0 ? (
            <div style={{ padding: 'var(--sp-4)' }}>
              <EmptyState
                title="No pipelines"
                hint="Create your first pipeline to get started."
              />
            </div>
          ) : (
            <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
              {pipelines.map((p) => {
                const active = p.id === selectedId;
                return (
                  <li
                    key={p.id}
                    onClick={() => setSelectedId(p.id)}
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
                    <div
                      className="mono"
                      style={{ fontSize: 'var(--fs-mono)', marginBottom: 4 }}
                    >
                      {p.name}
                    </div>
                    <div
                      style={{
                        display: 'flex',
                        gap: 'var(--sp-2)',
                        alignItems: 'center',
                        fontSize: 'var(--fs-caption)',
                        color: 'var(--color-text-secondary)',
                      }}
                    >
                      {p.target_type && <span>{p.target_type}</span>}
                      {p.last_run_status && (
                        <Badge tone={runTone(p.last_run_status)}>{p.last_run_status}</Badge>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {/* Right pane: run history */}
        <div>
          {selected ? (
            <RunHistory
              pipeline={selected}
              onViewRun={(id) => setViewingRun(id)}
            />
          ) : (
            <EmptyState
              title="Select a pipeline"
              hint="Pick one from the left to view its run history."
            />
          )}
        </div>
      </div>

      {creating && (
        <NewPipelineModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            toast('Pipeline created', 'success');
            reload();
          }}
        />
      )}

      {viewingRun && (
        <RunDetailModal runId={viewingRun} onClose={() => setViewingRun(null)} />
      )}
    </div>
  );
}

function RunHistory({
  pipeline,
  onViewRun,
}: {
  pipeline: Pipeline;
  onViewRun: (id: string) => void;
}) {
  const { data, loading, error } = useApi<{ data: Run[] }>(
    `pipelines/${pipeline.id}/runs`,
  );
  const runs: Run[] = data?.data ?? [];

  if (loading) {
    return (
      <div
        style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-5)',
          color: 'var(--color-text-muted)',
        }}
      >
        Loading runs…
      </div>
    );
  }
  if (error) {
    return (
      <div
        style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-5)',
          color: 'var(--color-error)',
        }}
      >
        {error}
      </div>
    );
  }

  const columns: Column<Run>[] = [
    {
      key: 'id',
      header: 'Run ID',
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
      render: (r) => fmtDate(r.started_at),
    },
    {
      key: 'duration',
      header: 'Duration',
      render: (r) => fmtDuration(r.duration_ms),
    },
    {
      key: 'triggered',
      header: 'Triggered By',
      render: (r) => r.triggered_by ?? '—',
    },
    {
      key: 'actions',
      header: 'Actions',
      render: (r) => (
        <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
          <Button variant="ghost" onClick={() => onViewRun(r.id)}>
            View
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 'var(--sp-3)',
        }}
      >
        <h2 style={{ fontSize: 'var(--fs-h2)' }}>{pipeline.name}</h2>
        {pipeline.target_type && (
          <Badge tone="info">{pipeline.target_type}</Badge>
        )}
      </div>
      <DataTable
        rows={runs}
        columns={columns}
        rowKey={(r) => r.id}
        empty={{
          title: 'No runs yet',
          hint: 'Click Trigger to start a new run.',
        }}
      />
    </div>
  );
}

function RunDetailModal({ runId, onClose }: { runId: string; onClose: () => void }) {
  const { push: toast } = useToast();
  const { data, loading, error, reload } = useApi<Run>(`runs/${runId}`);
  const [cancelling, setCancelling] = useState(false);

  async function handleCancel() {
    setCancelling(true);
    try {
      await apiPost(`runs/${runId}/cancel`);
      toast(`Run ${runId.slice(0, 8)} cancelled`, 'success');
      reload();
    } catch (e: any) {
      toast(`Cancel failed: ${e?.message ?? 'unknown'}`, 'error');
    } finally {
      setCancelling(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={`Run ${runId.slice(0, 12)}`}
      width={680}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
          {data && (data.status === 'running' || data.status === 'pending') && (
            <Button variant="danger" onClick={handleCancel} disabled={cancelling}>
              {cancelling ? 'Cancelling…' : 'Cancel Run'}
            </Button>
          )}
        </>
      }
    >
      {loading && <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>}
      {error && <div style={{ color: 'var(--color-error)' }}>{error}</div>}
      {data && (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '120px 1fr',
              gap: 'var(--sp-2) var(--sp-4)',
              marginBottom: 'var(--sp-5)',
              fontSize: 'var(--fs-small)',
            }}
          >
            <div style={{ color: 'var(--color-text-secondary)' }}>Status</div>
            <div><Badge tone={runTone(data.status)}>{data.status}</Badge></div>
            <div style={{ color: 'var(--color-text-secondary)' }}>Started</div>
            <div>{fmtDate(data.started_at)}</div>
            <div style={{ color: 'var(--color-text-secondary)' }}>Duration</div>
            <div>{fmtDuration(data.duration_ms)}</div>
            <div style={{ color: 'var(--color-text-secondary)' }}>Triggered By</div>
            <div>{data.triggered_by ?? '—'}</div>
          </div>

          <h3 style={{ fontSize: 'var(--fs-h3)', marginBottom: 'var(--sp-3)' }}>Steps</h3>
          {data.steps && data.steps.length > 0 ? (
            <ol style={{ listStyle: 'none', margin: 0, padding: 0 }}>
              {data.steps.map((s, i) => (
                <li
                  key={`${i}-${s.name}`}
                  style={{
                    background: 'var(--color-surface-elevated)',
                    border: '1px solid var(--color-border)',
                    borderRadius: 'var(--radius-sm)',
                    padding: 'var(--sp-3) var(--sp-4)',
                    marginBottom: 'var(--sp-2)',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    gap: 'var(--sp-3)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-3)' }}>
                    <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-caption)' }}>
                      {i + 1}.
                    </span>
                    <span className="mono">{s.name}</span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-3)' }}>
                    <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>
                      {fmtDuration(s.duration_ms)}
                    </span>
                    <Badge tone={runTone(s.status)}>{s.status}</Badge>
                  </div>
                </li>
              ))}
            </ol>
          ) : (
            <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-small)' }}>
              No step data available.
            </div>
          )}
        </>
      )}
    </Modal>
  );
}

function NewPipelineModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState('');
  const [targetType, setTargetType] = useState('k8s');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost('pipelines', {
        name,
        target_type: targetType,
        description,
      });
      onCreated();
    } catch (e: any) {
      setErr(e?.message ?? 'create failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="New Pipeline"
      width={480}
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
        {(s) => <input style={s} value={name} onChange={(e) => setName(e.target.value)} placeholder="build-and-deploy" />}
      </FormField>
      <FormField label="Target Type">
        {(s) => (
          <select style={s} value={targetType} onChange={(e) => setTargetType(e.target.value)}>
            <option value="k8s">k8s</option>
            <option value="vm">vm</option>
            <option value="physical">physical</option>
            <option value="serverless">serverless</option>
          </select>
        )}
      </FormField>
      <FormField label="Description">
        {(s) => <input style={s} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" />}
      </FormField>
    </Modal>
  );
}
