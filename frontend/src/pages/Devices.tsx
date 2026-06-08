// Devices page — list of devices with type/state filters, a probe action
// and a maintenance-toggle action (with reason modal for online devices).
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge, toneForDeviceState } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { FormField } from '../components/common/FormField';
import { useToast } from '../components/common/Toast';
import { useApi } from '../hooks/useApi';
import { apiPost, ListResponse } from '../api/client';

interface Device {
  id: number;
  name: string;
  type: string;
  state: string;
  group_id?: number | null;
  group_name?: string | null;
  labels?: Record<string, string> | null;
  ip?: string;
  ssh_port?: number;
  maintenance_reason?: string | null;
  maintenance_started_at?: string | null;
}

const STATE_OPTIONS = [
  { value: '', label: 'All states' },
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

export function Devices() {
  const toast = useToast();
  const [typeFilter, setTypeFilter] = useState('');
  const [stateFilter, setStateFilter] = useState('');
  const [maintenanceFor, setMaintenanceFor] = useState<Device | null>(null);
  const [reason, setReason] = useState('');
  const [busyId, setBusyId] = useState<number | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({ name: '', type: '', ip: '', ssh_port: '22' });
  const [createSubmitting, setCreateSubmitting] = useState(false);

  const query = useMemo(() => {
    const q: Record<string, string | number | undefined> = { limit: 200, offset: 0 };
    if (typeFilter) q.type = typeFilter;
    if (stateFilter) q.state = stateFilter;
    return q;
  }, [typeFilter, stateFilter]);

  const { data, loading, error, reload } = useApi<ListResponse<Device>>('devices', query);

  const types = useMemo(() => {
    const set = new Set<string>();
    (data?.data ?? []).forEach((d) => d.type && set.add(d.type));
    return Array.from(set).sort();
  }, [data]);

  const probe = async (d: Device) => {
    setBusyId(d.id);
    try {
      await apiPost(`devices/${d.id}/actions`, { action: 'probe' });
      toast.push(`Probe queued for ${d.name}`, 'success');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Probe failed', 'error');
    } finally {
      setBusyId(null);
    }
  };

  const submitMaintenance = async () => {
    if (!maintenanceFor) return;
    if (!reason.trim()) {
      toast.push('Reason is required', 'error');
      return;
    }
    setBusyId(maintenanceFor.id);
    try {
      await apiPost(`devices/${maintenanceFor.id}/actions`, {
        action: 'enter_maintenance',
        reason: reason.trim(),
      });
      toast.push(`${maintenanceFor.name} entered maintenance`, 'success');
      setMaintenanceFor(null);
      setReason('');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to enter maintenance', 'error');
    } finally {
      setBusyId(null);
    }
  };

  const exitMaintenance = async (d: Device) => {
    setBusyId(d.id);
    try {
      await apiPost(`devices/${d.id}/actions`, { action: 'exit_maintenance' });
      toast.push(`${d.name} exited maintenance`, 'success');
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to exit maintenance', 'error');
    } finally {
      setBusyId(null);
    }
  };

  const submitCreate = async () => {
    if (!createForm.name.trim() || !createForm.type.trim()) {
      toast.push('Name and Type are required', 'error');
      return;
    }
    setCreateSubmitting(true);
    try {
      const body: Record<string, unknown> = {
        name: createForm.name.trim(),
        type: createForm.type.trim(),
      };
      if (createForm.ip.trim()) body.ip = createForm.ip.trim();
      if (createForm.ssh_port) body.ssh_port = Number(createForm.ssh_port);
      await apiPost('devices', body);
      toast.push('Device created', 'success');
      setCreateOpen(false);
      setCreateForm({ name: '', type: '', ip: '', ssh_port: '22' });
      reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Failed to create device', 'error');
    } finally {
      setCreateSubmitting(false);
    }
  };

  // Refetch when filters change.
  useEffect(() => { reload(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [typeFilter, stateFilter]);

  const columns: Column<Device>[] = [
    { key: 'name', header: 'Name', render: (d) => <span className="mono">{d.name}</span>, width: 200 },
    { key: 'type', header: 'Type', render: (d) => <Badge tone="neutral">{d.type}</Badge> },
    { key: 'state', header: 'State', render: (d) => <Badge tone={toneForDeviceState(d.state)}>{d.state}</Badge> },
    { key: 'group', header: 'Group', render: (d) => d.group_name ?? (d.group_id ? `#${d.group_id}` : '—') },
    { key: 'labels', header: 'Labels', render: (d) => (
      <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
        {d.labels ? JSON.stringify(d.labels) : '—'}
      </span>
    ) },
    { key: 'actions', header: 'Actions', align: 'right', render: (d) => {
      const isBusy = busyId === d.id;
      return (
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--sp-2)' }}>
          <Button variant="ghost" disabled={isBusy} onClick={() => probe(d)}>Probe</Button>
          {d.state === 'maintenance' ? (
            <Button variant="secondary" disabled={isBusy} onClick={() => exitMaintenance(d)}>
              Exit Maintenance
            </Button>
          ) : (
            <Button variant="secondary" disabled={isBusy || d.state === 'offline'} onClick={() => {
              setReason('');
              setMaintenanceFor(d);
            }}>
              Maintenance
            </Button>
          )}
        </div>
      );
    } },
  ];

  return (
    <div>
      <PageHeader
        title="Devices"
        subtitle="Infrastructure devices under management"
        actions={<Button variant="primary" onClick={() => setCreateOpen(true)}>+ New Device</Button>}
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
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
          <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', textTransform: 'uppercase' }}>Type</span>
          <select style={selectStyle} value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
            <option value="">All types</option>
            {types.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
          <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', textTransform: 'uppercase' }}>State</span>
          <select style={selectStyle} value={stateFilter} onChange={(e) => setStateFilter(e.target.value)}>
            {STATE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div style={{ marginLeft: 'auto', fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)' }}>
          {data?.pagination ? `${data.pagination.total} total` : ''}
        </div>
      </div>

      {error && <div style={{ color: 'var(--color-error)', marginBottom: 'var(--sp-3)' }}>{error}</div>}

      <DataTable<Device>
        rows={data?.data ?? null}
        loading={loading}
        columns={columns}
        rowKey={(d) => String(d.id)}
        empty={{ title: 'No devices', hint: 'Try clearing the filters or add a new device.' }}
      />

      <Modal
        open={!!maintenanceFor}
        onClose={() => (busyId ? null : setMaintenanceFor(null))}
        title={`Enter Maintenance — ${maintenanceFor?.name ?? ''}`}
        footer={
          <>
            <Button variant="ghost" onClick={() => setMaintenanceFor(null)} disabled={busyId !== null}>Cancel</Button>
            <Button variant="primary" onClick={submitMaintenance} disabled={busyId !== null}>
              {busyId !== null ? 'Submitting…' : 'Enter Maintenance'}
            </Button>
          </>
        }
      >
        <p style={{ marginTop: 0, color: 'var(--color-text-secondary)' }}>
          External alerts will be suppressed while this device is in maintenance.
        </p>
        <FormField label="Reason" hint="Required. Briefly describe why maintenance is needed.">
          {(s) => (
            <textarea
              style={{ ...s, minHeight: 80, fontFamily: 'var(--font-mono)' }}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="e.g. OS upgrade window"
              maxLength={500}
            />
          )}
        </FormField>
      </Modal>

      <Modal
        open={createOpen}
        onClose={() => (createSubmitting ? null : setCreateOpen(false))}
        title="New Device"
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)} disabled={createSubmitting}>Cancel</Button>
            <Button variant="primary" onClick={submitCreate} disabled={createSubmitting}>
              {createSubmitting ? 'Creating…' : 'Create'}
            </Button>
          </>
        }
      >
        <FormField label="Name">
          {(s) => <input style={s} className="mono" value={createForm.name} onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })} placeholder="e.g. web-01" />}
        </FormField>
        <FormField label="Type" hint="e.g. server, switch, load-balancer">
          {(s) => <input style={s} value={createForm.type} onChange={(e) => setCreateForm({ ...createForm, type: e.target.value })} placeholder="server" />}
        </FormField>
        <FormField label="IP">
          {(s) => <input style={s} className="mono" value={createForm.ip} onChange={(e) => setCreateForm({ ...createForm, ip: e.target.value })} placeholder="10.0.0.1" />}
        </FormField>
        <FormField label="SSH Port">
          {(s) => <input style={s} className="mono" value={createForm.ssh_port} onChange={(e) => setCreateForm({ ...createForm, ssh_port: e.target.value })} placeholder="22" />}
        </FormField>
      </Modal>
    </div>
  );
}
