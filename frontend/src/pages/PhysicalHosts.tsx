// PhysicalHosts page — list of physical hosts with state filter,
// inline detail modal, and per-host probe + maintenance actions.
// The page subscribes to the device_event channel so a state
// change on the server refreshes the list (and the open detail
// Modal's metrics panel) without a manual reload.
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge, toneForDeviceState } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { FormField } from '../components/common/FormField';
import { MetricsPanel } from '../components/common/MetricsPanel';
import { useToast } from '../components/common/Toast';
import { useApi } from '../hooks/useApi';
import { useWebSocket } from '../hooks/useWebSocket';
import { apiPost, ListResponse } from '../api/client';

interface PhysicalHost {
  id: string;
  device_id: string;
  device_name: string;
  ip_address: string;
  ssh_port: number;
  ssh_user: string;
  state: string;
  last_check_at?: string | null;
  next_check_at?: string | null;
  consecutive_fails?: number;
  maintenance_started_at?: string | null;
  maintenance_reason?: string | null;
  maintenance_set_by?: string | null;
  created_at?: string;
  updated_at?: string;
}

const STATE_FILTERS = [
  { value: '', label: 'All' },
  { value: 'online', label: 'Online' },
  { value: 'monitoring_issue', label: 'Monitoring Issue' },
  { value: 'offline', label: 'Offline' },
  { value: 'maintenance', label: 'Maintenance' },
];

const selectStyle: React.CSSProperties = {
  background: 'var(--color-bg)',
  color: 'var(--color-text)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-sm)',
  padding: '6px 10px',
  fontSize: 'var(--fs-small)',
};

function relativeTime(iso?: string | null): string {
  if (!iso) return '—';
  const t = Date.parse(iso);
  if (isNaN(t)) return '—';
  const diff = Math.floor((Date.now() - t) / 1000);
  if (diff < 5) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function formatBytes(b?: number | null): string {
  if (b === null || b === undefined) return '—';
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
  if (b < 1024 * 1024 * 1024) return `${(b / 1024 / 1024).toFixed(1)} MB`;
  if (b < 1024 * 1024 * 1024 * 1024) return `${(b / 1024 / 1024 / 1024).toFixed(1)} GB`;
  return `${(b / 1024 / 1024 / 1024 / 1024).toFixed(1)} TB`;
}

export function PhysicalHosts() {
  const toast = useToast();
  const [stateFilter, setStateFilter] = useState('');
  const [selected, setSelected] = useState<PhysicalHost | null>(null);
  const [maintenanceFor, setMaintenanceFor] = useState<PhysicalHost | null>(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({ device_id: '', ip_address: '', ssh_port: '22', ssh_user: 'root' });
  const [createSubmitting, setCreateSubmitting] = useState(false);

  const query = useMemo(() => {
    const q: Record<string, string | number | undefined> = { limit: 200, offset: 0 };
    if (stateFilter) q.state = stateFilter;
    return q;
  }, [stateFilter]);

  const { data, loading, error, reload } = useApi<ListResponse<PhysicalHost>>('physical-hosts', query);

  useEffect(() => { reload(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [stateFilter]);

  // Tick once a minute to refresh relative times.
  useEffect(() => {
    const id = setInterval(() => { /* re-render */ }, 30000);
    return () => clearInterval(id);
  }, []);

  // WebSocket: when ANY physical_host.state_change event arrives,
  // refresh the list so the row colour and Last Check update.
  // The MetricsPanel reads its own refreshKey from the WS
  // arrival count below.
  const [wsTick, setWsTick] = useState(0);
  useWebSocket(['physical_host.state_change'], (e) => {
    if (e?.type === 'physical_host.state_change') {
      setWsTick((n) => n + 1);
      reload();
    }
  });

  const probe = async (h: PhysicalHost) => {
    setBusy(true);
    try {
      await apiPost(`physical-hosts/${h.id}/probe`);
      toast.push(`Probe queued for ${h.device_name}`, 'success');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Probe failed', 'error');
    } finally {
      setBusy(false);
    }
  };

  const enterMaintenance = async () => {
    if (!maintenanceFor) return;
    if (!reason.trim()) {
      toast.push('Reason is required', 'error');
      return;
    }
    setBusy(true);
    try {
      await apiPost(`physical-hosts/${maintenanceFor.id}/maintenance`, { reason: reason.trim() });
      toast.push(`${maintenanceFor.device_name} entered maintenance`, 'success');
      setMaintenanceFor(null);
      setReason('');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to enter maintenance', 'error');
    } finally {
      setBusy(false);
    }
  };

  const exitMaintenance = async (h: PhysicalHost) => {
    setBusy(true);
    try {
      await apiPost(`physical-hosts/${h.id}/maintenance/exit`);
      toast.push(`${h.device_name} exited maintenance`, 'success');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to exit maintenance', 'error');
    } finally {
      setBusy(false);
    }
  };

  const submitCreate = async () => {
    if (!createForm.device_id.trim() || !createForm.ip_address.trim()) {
      toast.push('device_id and ip_address are required', 'error');
      return;
    }
    setCreateSubmitting(true);
    try {
      const body: Record<string, unknown> = {
        device_id: createForm.device_id.trim(),
        ip_address: createForm.ip_address.trim(),
        ssh_port: Number(createForm.ssh_port || 22),
        ssh_user: createForm.ssh_user.trim() || 'root',
      };
      await apiPost('physical-hosts', body);
      toast.push('Host created', 'success');
      setCreateOpen(false);
      setCreateForm({ device_id: '', ip_address: '', ssh_port: '22', ssh_user: 'root' });
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to create host', 'error');
    } finally {
      setCreateSubmitting(false);
    }
  };

  const columns: Column<PhysicalHost>[] = [
    { key: 'name', header: 'Device', render: (h) => <span className="mono">{h.device_name}</span>, width: 200 },
    { key: 'ip', header: 'IP', render: (h) => <span className="mono">{h.ip_address}</span> },
    { key: 'ssh_port', header: 'SSH Port', render: (h) => <span className="mono">{h.ssh_port}</span> },
    { key: 'state', header: 'State', render: (h) => <Badge tone={toneForDeviceState(h.state)}>{h.state}</Badge> },
    { key: 'last_check', header: 'Last Check', render: (h) => relativeTime(h.last_check_at) },
    { key: 'actions', header: 'Actions', align: 'right', render: (h) => (
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--sp-2)' }}>
        <Button variant="ghost" disabled={busy} onClick={() => setSelected(h)}>View</Button>
      </div>
    ) },
  ];

  return (
    <div>
      <PageHeader
        title="Physical Hosts"
        subtitle="Bare-metal and VM hosts under management"
        actions={<Button variant="primary" onClick={() => setCreateOpen(true)}>+ New Host</Button>}
      />

      <div
        style={{
          display: 'flex',
          gap: 'var(--sp-3)',
          alignItems: 'center',
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-3) var(--sp-4)',
          marginBottom: 'var(--sp-4)',
        }}
      >
        <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', textTransform: 'uppercase' }}>State</span>
        <select style={selectStyle} value={stateFilter} onChange={(e) => setStateFilter(e.target.value)}>
          {STATE_FILTERS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
        <div style={{ marginLeft: 'auto', fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
          {data?.pagination ? `${data.pagination.total} total` : ''}
        </div>
      </div>

      {error && <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{error}</div>}

      <DataTable<PhysicalHost>
        rows={data?.data ?? null}
        loading={loading}
        columns={columns}
        rowKey={(h) => String(h.id)}
        empty={{ title: 'No hosts', hint: 'Try clearing the filter or add a new host.' }}
      />

      {/* Detail modal */}
      <Modal
        open={!!selected}
        onClose={() => (busy ? null : setSelected(null))}
        title={selected ? `${selected.device_name} — ${selected.ip_address}` : 'Host'}
        width={560}
        footer={
          selected ? (
            <div style={{ display: 'flex', gap: 'var(--sp-2)', width: '100%', justifyContent: 'space-between' }}>
              <Button variant="ghost" onClick={() => setSelected(null)} disabled={busy}>Close</Button>
              <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
                <Button variant="secondary" disabled={busy} onClick={() => probe(selected)}>Probe</Button>
                {selected.state === 'maintenance' ? (
                  <Button variant="primary" disabled={busy} onClick={() => exitMaintenance(selected)}>
                    Exit Maintenance
                  </Button>
                ) : (
                  <Button variant="primary" disabled={busy} onClick={() => {
                    setReason('');
                    setMaintenanceFor(selected);
                  }}>
                    Enter Maintenance
                  </Button>
                )}
              </div>
            </div>
          ) : null
        }
      >
        {selected && (
          <div>
            {selected.state === 'maintenance' && (
              <div
                style={{
                  background: 'rgba(168,85,247,0.1)',
                  border: '1px solid #a855f7',
                  borderRadius: 'var(--radius-sm)',
                  padding: 'var(--sp-2) var(--sp-3)',
                  marginBottom: 'var(--sp-4)',
                  fontSize: 'var(--fs-small)',
                  color: '#a855f7',
                }}
              >
                Maintenance since {selected.maintenance_started_at ? new Date(selected.maintenance_started_at).toLocaleString() : '—'}
                {selected.maintenance_reason ? ` — ${selected.maintenance_reason}` : ''}
              </div>
            )}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 'var(--sp-3)' }}>
              <Detail label="ID" value={<span className="mono">{selected.id}</span>} />
              <Detail label="State" value={<Badge tone={toneForDeviceState(selected.state)}>{selected.state}</Badge>} />
              <Detail label="Device" value={<span className="mono">{selected.device_name}</span>} />
              <Detail label="IP" value={<span className="mono">{selected.ip_address}</span>} />
              <Detail label="SSH Port" value={<span className="mono">{selected.ssh_port}</span>} />
              <Detail label="SSH User" value={<span className="mono">{selected.ssh_user}</span>} />
              <Detail label="Last Check" value={relativeTime(selected.last_check_at)} />
              <Detail label="Consecutive Fails" value={<span className="mono">{selected.consecutive_fails ?? 0}</span>} />
            </div>
            <MetricsPanel hostId={selected.id} refreshKey={wsTick} />
          </div>
        )}
      </Modal>

      {/* Maintenance reason modal */}
      <Modal
        open={!!maintenanceFor}
        onClose={() => (busy ? null : setMaintenanceFor(null))}
        title={`Enter Maintenance — ${maintenanceFor?.device_name ?? ''}`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setMaintenanceFor(null)} disabled={busy}>Cancel</Button>
            <Button variant="primary" onClick={enterMaintenance} disabled={busy}>
              {busy ? 'Submitting…' : 'Enter Maintenance'}
            </Button>
          </>
        }
      >
        <p style={{ marginTop: 0, color: 'var(--color-text-secondary)' }}>
          External alerts will be suppressed while this host is in maintenance.
        </p>
        <FormField label="Reason" hint="Required. Briefly describe why maintenance is needed.">
          {(s) => (
            <textarea
              style={{ ...s, minHeight: 80, fontFamily: 'var(--font-mono)' }}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="e.g. Database schema upgrade"
              maxLength={500}
            />
          )}
        </FormField>
      </Modal>

      {/* Create modal */}
      <Modal
        open={createOpen}
        onClose={() => (createSubmitting ? null : setCreateOpen(false))}
        title="New Physical Host"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)} disabled={createSubmitting}>Cancel</Button>
            <Button variant="primary" onClick={submitCreate} disabled={createSubmitting}>
              {createSubmitting ? 'Creating…' : 'Create'}
            </Button>
          </>
        }
      >
        <FormField label="Device ID">
          {(s) => <input style={s} className="mono" value={createForm.device_id} onChange={(e) => setCreateForm({ ...createForm, device_id: e.target.value })} placeholder="device-uuid" />}
        </FormField>
        <FormField label="IP Address">
          {(s) => <input style={s} className="mono" value={createForm.ip_address} onChange={(e) => setCreateForm({ ...createForm, ip_address: e.target.value })} placeholder="10.0.0.1" />}
        </FormField>
        <FormField label="SSH User">
          {(s) => <input style={s} className="mono" value={createForm.ssh_user} onChange={(e) => setCreateForm({ ...createForm, ssh_user: e.target.value })} placeholder="root" />}
        </FormField>
        <FormField label="SSH Port">
          {(s) => <input style={s} className="mono" value={createForm.ssh_port} onChange={(e) => setCreateForm({ ...createForm, ssh_port: e.target.value })} placeholder="22" />}
        </FormField>
      </Modal>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: 0.5, marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 'var(--fs-small)' }}>{value}</div>
    </div>
  );
}
