// Alerts — three sub-views: Active, Channels, History. Real-time prepend from WS.
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge, toneForSeverity } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { Modal } from '../components/common/Modal';
import { FormField } from '../components/common/FormField';
import { useApi } from '../hooks/useApi';
import { apiPost } from '../api/client';
import { useWebSocket, WsEvent } from '../hooks/useWebSocket';
import { useToast } from '../components/common/Toast';

type Tab = 'active' | 'channels' | 'history';

interface Alert {
  id: string;
  name: string;
  severity: string;
  source: string;
  state: string;
  fired_at: string;
  [k: string]: any;
}

interface Channel {
  id: string;
  type: string;
  name: string;
  enabled: boolean;
  config?: Record<string, any>;
  [k: string]: any;
}

interface HistoryItem {
  id: string;
  fired_at: string;
  resolved_at?: string;
  name: string;
  severity: string;
  source: string;
  suppressed?: boolean;
  [k: string]: any;
}

export function Alerts() {
  const toast = useToast();
  const [tab, setTab] = useState<Tab>('active');
  const [liveAlerts, setLiveAlerts] = useState<Alert[]>([]);
  const [showSuppressed, setShowSuppressed] = useState(false);
  const [showChannelModal, setShowChannelModal] = useState(false);

  // Active alerts
  const active = useApi<Alert[] | { data: Alert[] }>('alerts');

  // Channels
  const channels = useApi<Channel[] | { data: Channel[] }>('alerts/channels');

  // History
  const history = useApi<HistoryItem[] | { data: HistoryItem[] }>(
    'alerts/history',
    showSuppressed ? { suppressed: true } : undefined,
  );

  // Live WS subscription
  useWebSocket(['alerts.fired', 'alerts.resolved'], (e: WsEvent) => {
    if (e.channel !== 'alerts.fired' && e.channel !== 'alerts.resolved') return;
    const a: Alert = {
      id: e.data?.id ?? `live-${Date.now()}-${Math.random()}`,
      name: e.data?.name ?? 'Alert',
      severity: e.data?.severity ?? 'info',
      source: e.data?.source ?? 'unknown',
      state: e.channel === 'alerts.fired' ? 'firing' : 'resolved',
      fired_at: e.data?.fired_at ?? e.timestamp,
      ...e.data,
    };
    setLiveAlerts((cur) => {
      // Replace any existing row with same id; otherwise prepend.
      const without = cur.filter((x) => x.id !== a.id);
      return [a, ...without].slice(0, 200);
    });
  });

  const activeRows: Alert[] = useMemo(() => {
    const fromApi: any = active.data;
    const apiList: Alert[] = Array.isArray(fromApi) ? fromApi : (fromApi?.data ?? []);
    const seen = new Set<string>();
    const merged: Alert[] = [];
    for (const a of liveAlerts) {
      if (!seen.has(a.id)) { merged.push(a); seen.add(a.id); }
    }
    for (const a of apiList) {
      if (!seen.has(a.id)) { merged.push(a); seen.add(a.id); }
    }
    return merged;
  }, [active.data, liveAlerts]);

  const channelRows: Channel[] = useMemo(() => {
    const d: any = channels.data;
    return Array.isArray(d) ? d : (d?.data ?? []);
  }, [channels.data]);

  const historyRows: HistoryItem[] = useMemo(() => {
    const d: any = history.data;
    return Array.isArray(d) ? d : (d?.data ?? []);
  }, [history.data]);

  const onAck = async (id: string) => {
    try {
      await apiPost(`alerts/${encodeURIComponent(id)}/acknowledge`);
      setLiveAlerts((cur) => cur.map((a) => a.id === id ? { ...a, state: 'acknowledged' } : a));
      toast.push('Alert acknowledged', 'success');
      active.reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Acknowledge failed', 'error');
    }
  };

  const onResolve = async (id: string) => {
    try {
      await apiPost(`alerts/${encodeURIComponent(id)}/resolve`);
      setLiveAlerts((cur) => cur.map((a) => a.id === id ? { ...a, state: 'resolved' } : a));
      toast.push('Alert resolved', 'success');
      active.reload();
    } catch (e: any) {
      toast.push(e?.message ?? 'Resolve failed', 'error');
    }
  };

  const activeColumns: Column<Alert>[] = [
    {
      key: 'severity',
      header: 'Severity',
      width: 110,
      render: (a) => <Badge tone={toneForSeverity((a.severity ?? '').toLowerCase())}>{a.severity ?? '—'}</Badge>,
    },
    { key: 'name', header: 'Name', render: (a) => <span className="mono">{a.name}</span> },
    { key: 'source', header: 'Source', width: 160, render: (a) => <span style={{ fontSize: 'var(--fs-caption)' }}>{a.source}</span> },
    {
      key: 'state',
      header: 'State',
      width: 140,
      render: (a) => <Badge tone={a.state === 'firing' ? 'error' : a.state === 'acknowledged' ? 'warning' : a.state === 'resolved' ? 'success' : 'neutral'}>{a.state}</Badge>,
    },
    {
      key: 'fired',
      header: 'Fired At',
      width: 200,
      render: (a) => <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>{a.fired_at}</span>,
    },
    {
      key: 'actions',
      header: 'Actions',
      width: 240,
      align: 'right',
      render: (a) => (
        <div style={{ display: 'inline-flex', gap: 'var(--sp-2)' }}>
          <Button
            variant="secondary"
            onClick={() => onAck(a.id)}
            disabled={a.state === 'acknowledged' || a.state === 'resolved'}
          >Acknowledge</Button>
          <Button
            variant="primary"
            onClick={() => onResolve(a.id)}
            disabled={a.state === 'resolved'}
          >Resolve</Button>
        </div>
      ),
    },
  ];

  const channelColumns: Column<Channel>[] = [
    { key: 'type', header: 'Type', width: 140, render: (c) => <Badge tone="primary">{c.type}</Badge> },
    { key: 'name', header: 'Name', render: (c) => <span className="mono">{c.name}</span> },
    {
      key: 'enabled',
      header: 'Enabled',
      width: 110,
      render: (c) => <Badge tone={c.enabled ? 'success' : 'muted'}>{c.enabled ? 'enabled' : 'disabled'}</Badge>,
    },
  ];

  const historyColumns: Column<HistoryItem>[] = [
    {
      key: 'fired',
      header: 'Fired At',
      width: 200,
      render: (h) => <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>{h.fired_at}</span>,
    },
    {
      key: 'sev',
      header: 'Severity',
      width: 110,
      render: (h) => <Badge tone={toneForSeverity((h.severity ?? '').toLowerCase())}>{h.severity ?? '—'}</Badge>,
    },
    { key: 'name', header: 'Name', render: (h) => <span className="mono">{h.name}</span> },
    { key: 'source', header: 'Source', width: 160, render: (h) => <span style={{ fontSize: 'var(--fs-caption)' }}>{h.source}</span> },
    {
      key: 'suppressed',
      header: 'Suppressed',
      width: 120,
      render: (h) => h.suppressed ? <Badge tone="muted">suppressed</Badge> : <Badge tone="neutral">—</Badge>,
    },
  ];

  return (
    <div>
      <PageHeader
        title="Alerts"
        subtitle="Active alerts, notification channels, and history"
        actions={tab === 'channels' ? <Button variant="primary" onClick={() => setShowChannelModal(true)}>+ New Channel</Button> : undefined}
      />

      <div style={{
        display: 'flex',
        gap: 'var(--sp-1)',
        borderBottom: '1px solid var(--color-border)',
        marginBottom: 'var(--sp-5)',
      }}>
        {(['active', 'channels', 'history'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              background: 'transparent',
              border: 'none',
              borderBottom: tab === t ? '2px solid var(--color-primary)' : '2px solid transparent',
              color: tab === t ? 'var(--color-primary)' : 'var(--color-text-secondary)',
              padding: 'var(--sp-3) var(--sp-4)',
              fontSize: 'var(--fs-small)',
              fontWeight: tab === t ? 600 : 500,
              cursor: 'pointer',
              textTransform: 'capitalize',
            }}
          >{t}</button>
        ))}
      </div>

      {tab === 'active' && (
        <DataTable
          rows={activeRows}
          columns={activeColumns}
          rowKey={(a) => a.id}
          loading={active.loading}
          empty={{ title: 'No active alerts', hint: 'You are all clear.' }}
        />
      )}

      {tab === 'channels' && (
        <DataTable
          rows={channelRows}
          columns={channelColumns}
          rowKey={(c) => c.id ?? c.name}
          loading={channels.loading}
          empty={{ title: 'No channels', hint: 'Add a notification channel to receive alerts.' }}
        />
      )}

      {tab === 'history' && (
        <>
          <div style={{ marginBottom: 'var(--sp-3)', display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 'var(--fs-small)', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
              <input type="checkbox" checked={showSuppressed} onChange={(e) => setShowSuppressed(e.target.checked)} />
              Show suppressed
            </label>
          </div>
          <DataTable
            rows={historyRows}
            columns={historyColumns}
            rowKey={(h) => h.id}
            loading={history.loading}
            empty={{ title: 'No history', hint: 'No resolved alerts in the selected window.' }}
          />
        </>
      )}

      <NewChannelModal
        open={showChannelModal}
        onClose={() => setShowChannelModal(false)}
        onCreated={() => { channels.reload(); setShowChannelModal(false); }}
      />
    </div>
  );
}

function NewChannelModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [type, setType] = useState('webhook');
  const [name, setName] = useState('');
  const [configJson, setConfigJson] = useState('{\n  "url": ""\n}');
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async () => {
    let config: Record<string, any> = {};
    try { config = JSON.parse(configJson); } catch {
      toast.push('Config must be valid JSON', 'error');
      return;
    }
    if (!name.trim()) {
      toast.push('Name is required', 'error');
      return;
    }
    setSubmitting(true);
    try {
      await apiPost('alerts/channels', { type, name: name.trim(), enabled: true, config });
      toast.push('Channel created', 'success');
      onCreated();
    } catch (e: any) {
      toast.push(e?.message ?? 'Create failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New Channel"
      footer={
        <>
          <Button onClick={onClose} disabled={submitting}>Cancel</Button>
          <Button variant="primary" onClick={onSubmit} disabled={submitting}>{submitting ? 'Creating…' : 'Create'}</Button>
        </>
      }
    >
      <FormField label="Type">
        {(s) => (
          <select style={s} value={type} onChange={(e) => setType(e.target.value)}>
            <option value="webhook">webhook</option>
            <option value="email">email</option>
            <option value="slack">slack</option>
            <option value="pagerduty">pagerduty</option>
          </select>
        )}
      </FormField>
      <FormField label="Name">
        {(s) => <input style={s} value={name} onChange={(e) => setName(e.target.value)} placeholder="oncall-webhook" />}
      </FormField>
      <FormField label="Config (JSON)">
        {(s) => <textarea style={{ ...s, fontFamily: 'var(--font-mono)', minHeight: 120 }} value={configJson} onChange={(e) => setConfigJson(e.target.value)} />}
      </FormField>
    </Modal>
  );
}
