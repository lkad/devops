// Discovery — list page of discovery runs (network scans).
// Top: "+ New Scan" Button.
// Body: DataTable (CIDR, started_at, status, hosts_found, actions).
// New Scan: modal with form (CIDR, ports, snmp).
// Promote (when run is complete): modal with discovered hosts, multi-select,
// POST /api/v1/discovery/runs/:id/promote with {host_ids}.

import { useState } from 'react';
import { useApi } from '../hooks/useApi';
import { apiPost } from '../api/client';
import { Badge } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { EmptyState } from '../components/common/EmptyState';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';

type RunStatus = 'pending' | 'running' | 'completed' | 'failed' | 'complete' | 'cancelled' | string;

interface DiscoveryRun {
  id: string;
  cidr: string;
  status: RunStatus;
  started_at?: string;
  finished_at?: string;
  hosts_found?: number;
  ports?: string;
  snmp?: boolean;
  hosts?: DiscoveredHost[];
}

interface DiscoveredHost {
  id: string;
  ip: string;
  hostname?: string;
  mac?: string;
  vendor?: string;
  open_ports?: number[];
}

function statusTone(s: RunStatus) {
  if (s === 'completed' || s === 'complete') return 'success' as const;
  if (s === 'failed' || s === 'error') return 'error' as const;
  if (s === 'running') return 'info' as const;
  if (s === 'cancelled') return 'muted' as const;
  if (s === 'pending') return 'warning' as const;
  return 'neutral' as const;
}

function fmtDate(s?: string) {
  if (!s) return '—';
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}

export function Discovery() {
  const [creating, setCreating] = useState(false);
  const [promotingRun, setPromotingRun] = useState<DiscoveryRun | null>(null);
  const { push: toast } = useToast();
  const { data, loading, error, reload } = useApi<{ data: DiscoveryRun[] }>('discovery/runs');
  const runs: DiscoveryRun[] = data?.data ?? [];

  const columns: Column<DiscoveryRun>[] = [
    {
      key: 'cidr',
      header: 'CIDR',
      render: (r) => <span className="mono">{r.cidr}</span>,
    },
    {
      key: 'started',
      header: 'Started',
      render: (r) => fmtDate(r.started_at),
    },
    {
      key: 'status',
      header: 'Status',
      render: (r) => <Badge tone={statusTone(r.status)}>{r.status}</Badge>,
    },
    {
      key: 'hosts',
      header: 'Hosts Found',
      render: (r) => (r.hosts_found ?? (r.hosts?.length ?? 0)).toString(),
      align: 'right',
    },
    {
      key: 'actions',
      header: 'Actions',
      render: (r) => {
        const complete = r.status === 'completed' || r.status === 'complete';
        return (
          <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
            <Button
              variant="primary"
              onClick={() => setPromotingRun(r)}
              disabled={!complete}
            >
              Promote
            </Button>
          </div>
        );
      },
    },
  ];

  return (
    <div>
      <PageHeader
        title="Discovery"
        subtitle="Network scans and discovered hosts"
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            + New Scan
          </Button>
        }
      />

      {error && <div style={{ color: 'var(--color-error)' }}>{error}</div>}

      {loading ? (
        <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>
      ) : (
        <DataTable
          rows={runs}
          columns={columns}
          rowKey={(r) => r.id}
          empty={{
            title: 'No discovery runs',
            hint: 'Click "+ New Scan" to scan a network range.',
          }}
        />
      )}

      {creating && (
        <NewScanModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            toast('Scan started', 'success');
            reload();
          }}
        />
      )}

      {promotingRun && (
        <PromoteModal
          run={promotingRun}
          onClose={() => setPromotingRun(null)}
          onPromoted={() => {
            setPromotingRun(null);
            toast('Hosts promoted to Devices', 'success');
            reload();
          }}
        />
      )}
    </div>
  );
}

function NewScanModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [cidr, setCidr] = useState('10.0.0.0/24');
  const [ports, setPorts] = useState('22,80,443');
  const [snmp, setSnmp] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      await apiPost('discovery/runs', { cidr, ports, snmp });
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
      title="New Scan"
      width={460}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit} disabled={submitting || !cidr}>
            {submitting ? 'Starting…' : 'Start Scan'}
          </Button>
        </>
      }
    >
      {err && <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{err}</div>}
      <FormField label="CIDR" hint="e.g. 10.0.0.0/24">
        {(s) => <input style={s} value={cidr} onChange={(e) => setCidr(e.target.value)} placeholder="10.0.0.0/24" />}
      </FormField>
      <FormField label="Ports" hint="Comma-separated, e.g. 22,80,443">
        {(s) => <input style={s} value={ports} onChange={(e) => setPorts(e.target.value)} placeholder="22,80,443" />}
      </FormField>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--sp-2)',
          marginBottom: 'var(--sp-4)',
        }}
      >
        <input
          id="snmp-cb"
          type="checkbox"
          checked={snmp}
          onChange={(e) => setSnmp(e.target.checked)}
        />
        <label
          htmlFor="snmp-cb"
          style={{ fontSize: 'var(--fs-small)', color: 'var(--color-text)' }}
        >
          Use SNMP for device fingerprinting
        </label>
      </div>
    </Modal>
  );
}

function PromoteModal({
  run,
  onClose,
  onPromoted,
}: {
  run: DiscoveryRun;
  onClose: () => void;
  onPromoted: () => void;
}) {
  // If hosts aren't embedded, fetch the full run.
  const { data: detail } = useApi<DiscoveryRun>(`discovery/runs/${run.id}`);
  const full = detail ?? run;
  const hosts = full.hosts ?? [];
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function toggle(id: string) {
    setSelected((cur) => {
      const next = new Set(cur);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (selected.size === hosts.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(hosts.map((h) => h.id)));
    }
  }

  async function submit() {
    setSubmitting(true);
    setErr(null);
    try {
      const body: { host_ids: string[] } = { host_ids: Array.from(selected) };
      await apiPost(`discovery/runs/${run.id}/promote`, body);
      onPromoted();
    } catch (e: any) {
      setErr(e?.message ?? 'promote failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={`Promote Hosts — ${run.cidr}`}
      width={680}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            onClick={submit}
            disabled={submitting || selected.size === 0}
          >
            {submitting
              ? 'Promoting…'
              : `Promote ${selected.size > 0 ? `(${selected.size})` : ''}`}
          </Button>
        </>
      }
    >
      {err && <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{err}</div>}

      {hosts.length === 0 ? (
        <EmptyState
          title="No hosts discovered"
          hint="This scan did not return any host records."
        />
      ) : (
        <>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: 'var(--sp-3)',
            }}
          >
            <span style={{ fontSize: 'var(--fs-small)', color: 'var(--color-text-secondary)' }}>
              {hosts.length} host{hosts.length === 1 ? '' : 's'} found
            </span>
            <Button variant="ghost" onClick={toggleAll}>
              {selected.size === hosts.length ? 'Deselect all' : 'Select all'}
            </Button>
          </div>
          <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
            {hosts.map((h) => {
              const checked = selected.has(h.id);
              return (
                <li
                  key={h.id}
                  onClick={() => toggle(h.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 'var(--sp-3)',
                    padding: 'var(--sp-2) var(--sp-3)',
                    border: '1px solid var(--color-border)',
                    borderRadius: 'var(--radius-sm)',
                    marginBottom: 'var(--sp-2)',
                    background: checked
                      ? 'var(--color-primary-muted)'
                      : 'var(--color-surface-elevated)',
                    cursor: 'pointer',
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggle(h.id)}
                    onClick={(e) => e.stopPropagation()}
                  />
                  <div style={{ flex: 1 }}>
                    <div className="mono">{h.ip}</div>
                    <div
                      style={{
                        fontSize: 'var(--fs-caption)',
                        color: 'var(--color-text-secondary)',
                      }}
                    >
                      {[h.hostname, h.vendor, h.mac].filter(Boolean).join(' · ') || '—'}
                    </div>
                  </div>
                  {h.open_ports && h.open_ports.length > 0 && (
                    <div
                      style={{
                        fontSize: 'var(--fs-caption)',
                        color: 'var(--color-text-muted)',
                      }}
                    >
                      {h.open_ports.length} port{h.open_ports.length === 1 ? '' : 's'}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
    </Modal>
  );
}
