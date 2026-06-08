// Logs — query form, capabilities card, results table, optional live stream.
import { useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { DataTable, Column } from '../components/common/DataTable';
import { Badge, toneForSeverity } from '../components/common/Badge';
import { Button } from '../components/common/Button';
import { FormField } from '../components/common/FormField';
import { useApi } from '../hooks/useApi';
import { apiGet } from '../api/client';
import { useWebSocket, WsEvent } from '../hooks/useWebSocket';
import { useToast } from '../components/common/Toast';

interface LogCapabilities {
  backend: string;
  capabilities: {
    supports_aggregation: boolean;
    max_time_range: string;
    max_query_length: number;
    backend_name: string;
  };
}

interface LogEntry {
  id: string;
  timestamp: string;
  level: string;
  source: string;
  host?: string;
  message: string;
  [k: string]: any;
}

interface QueryMeta {
  backend: string;
  degraded: boolean;
  reason: string;
  limits: Record<string, any>;
}

interface QueryResponse {
  data: LogEntry[];
  meta: QueryMeta;
}

function toLocalInput(iso: string): string {
  // Strip the seconds / TZ to fit a datetime-local input.
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalInput(s: string): string {
  // Treat a datetime-local value as UTC.
  if (!s) return '';
  return new Date(s).toISOString();
}

const STREAM_MAX = 500;
const TABLE_LIMIT = 200;

export function Logs() {
  const toast = useToast();

  // Form state
  const [q, setQ] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [limit, setLimit] = useState(100);

  // Submitted query parameters (drive the GET request)
  const [submitted, setSubmitted] = useState<{ q: string; from: string; to: string; limit: number } | null>(null);

  // Live stream rows (prepended, capped)
  const [streamRows, setStreamRows] = useState<LogEntry[]>([]);
  const [streamOn, setStreamOn] = useState(false);

  // Capabilities
  const caps = useApi<LogCapabilities>('logs/capabilities');

  // Build query
  const queryKey = useMemo(() => {
    if (!submitted) return null;
    const params: Record<string, any> = { limit: submitted.limit };
    if (submitted.q) params.q = submitted.q;
    if (submitted.from) params.from = submitted.from;
    if (submitted.to) params.to = submitted.to;
    return params;
  }, [submitted]);

  const result = useApi<QueryResponse>(queryKey ? 'logs/query' : null, queryKey ?? undefined);

  // WebSocket subscription (only when stream is on)
  const wsChannels = streamOn ? ['logs.*', 'container_log'] : [];
  useWebSocket(wsChannels, (e: WsEvent) => {
    if (!streamOn) return;
    if (e.channel !== 'logs.*' && e.channel !== 'container_log') return;
    const d = e.data;
    if (!d || typeof d !== 'object') return;
    const row: LogEntry = {
      id: d.id ?? `${e.timestamp}-${Math.random()}`,
      timestamp: d.timestamp ?? e.timestamp,
      level: d.level ?? 'info',
      source: d.source ?? (e.channel === 'container_log' ? 'container' : 'stream'),
      host: d.host,
      message: d.message ?? (typeof d === 'string' ? d : JSON.stringify(d)),
      ...d,
    };
    setStreamRows((cur) => [row, ...cur].slice(0, STREAM_MAX));
  });

  // Merge stream + query rows, dedupe by id, cap.
  const rows: LogEntry[] = useMemo(() => {
    const fromApi = (result.data?.data ?? []).slice(0, TABLE_LIMIT);
    const seen = new Set<string>();
    const merged: LogEntry[] = [];
    for (const r of streamRows) {
      if (!seen.has(r.id)) { merged.push(r); seen.add(r.id); }
      if (merged.length >= TABLE_LIMIT) break;
    }
    if (merged.length < TABLE_LIMIT) {
      for (const r of fromApi) {
        if (!seen.has(r.id)) { merged.push(r); seen.add(r.id); }
        if (merged.length >= TABLE_LIMIT) break;
      }
    }
    return merged;
  }, [result.data, streamRows]);

  const meta = result.data?.meta;

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitted({ q, from: fromLocalInput(from), to: fromLocalInput(to), limit });
  };

  const onReset = () => {
    setQ(''); setFrom(''); setTo(''); setLimit(100);
    setSubmitted(null);
  };

  const columns: Column<LogEntry>[] = [
    {
      key: 'ts',
      header: 'Timestamp',
      width: 200,
      render: (r) => <span className="mono" style={{ color: 'var(--color-text-secondary)' }}>{r.timestamp}</span>,
    },
    {
      key: 'level',
      header: 'Level',
      width: 100,
      render: (r) => <Badge tone={toneForSeverity((r.level ?? '').toLowerCase())}>{r.level ?? '—'}</Badge>,
    },
    {
      key: 'source',
      header: 'Source',
      width: 140,
      render: (r) => <span className="mono" style={{ fontSize: 'var(--fs-caption)' }}>{r.source ?? '—'}</span>,
    },
    {
      key: 'host',
      header: 'Host',
      width: 160,
      render: (r) => <span className="mono" style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>{r.host ?? '—'}</span>,
    },
    {
      key: 'message',
      header: 'Message',
      render: (r) => <span style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-caption)' }}>{r.message}</span>,
    },
  ];

  return (
    <div>
      <PageHeader
        title="Logs"
        subtitle="Search and tail logs across all backends"
        actions={
          <Button
            variant={streamOn ? 'primary' : 'secondary'}
            onClick={() => setStreamOn((s) => !s)}
          >{streamOn ? '■ Stop Stream' : '▶ Stream'}</Button>
        }
      />

      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 'var(--sp-4)', marginBottom: 'var(--sp-5)' }}>
        <form onSubmit={onSubmit} style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-4)',
        }}>
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 120px auto', gap: 'var(--sp-3)', alignItems: 'end' }}>
            <FormField label="Query (q)">
              {(s) => <input style={s} value={q} onChange={(e) => setQ(e.target.value)} placeholder="error OR timeout" />}
            </FormField>
            <FormField label="From">
              {(s) => <input style={s} type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} />}
            </FormField>
            <FormField label="To">
              {(s) => <input style={s} type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} />}
            </FormField>
            <FormField label="Limit">
              {(s) => <input style={s} type="number" min={1} max={1000} value={limit} onChange={(e) => setLimit(Number(e.target.value) || 100)} />}
            </FormField>
            <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
              <Button type="submit" variant="primary">Search</Button>
              <Button type="button" onClick={onReset}>Reset</Button>
            </div>
          </div>
        </form>

        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-4)',
        }}>
          <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600, marginBottom: 'var(--sp-3)' }}>
            Capabilities
          </div>
          {caps.loading && <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-small)' }}>Loading…</div>}
          {caps.error && <div style={{ color: 'var(--color-error)', fontSize: 'var(--fs-small)' }}>{caps.error}</div>}
          {caps.data && (
            <div style={{ display: 'grid', gap: 'var(--sp-2)', fontSize: 'var(--fs-small)' }}>
              <Row label="Backend" value={caps.data.capabilities?.backend_name ?? caps.data.backend} />
              <Row label="Aggregation" value={caps.data.capabilities?.supports_aggregation ? 'supported' : 'not supported'} />
              <Row label="Max time range" value={caps.data.capabilities?.max_time_range || 'unlimited'} />
              <Row label="Max query length" value={String(caps.data.capabilities?.max_query_length ?? '—')} />
            </div>
          )}
        </div>
      </div>

      {meta?.degraded && (
        <div style={{
          background: 'rgba(168, 85, 247, 0.1)',
          border: '1px solid #a855f7',
          borderLeft: '3px solid #a855f7',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-3) var(--sp-4)',
          marginBottom: 'var(--sp-4)',
          fontSize: 'var(--fs-small)',
          color: 'var(--color-text)',
        }}>
          <strong style={{ color: '#a855f7' }}>Degraded:</strong>{' '}
          Backend: <span className="mono">{meta.backend}</span> | Degraded: <span>{meta.reason || 'see server'}</span>
        </div>
      )}

      <DataTable
        rows={rows}
        columns={columns}
        rowKey={(r) => r.id}
        loading={result.loading}
        empty={{ title: 'No logs', hint: submitted ? 'Try a different query or time range.' : 'Submit a search to begin.' }}
      />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 'var(--sp-2)' }}>
      <span style={{ color: 'var(--color-text-secondary)' }}>{label}</span>
      <span className="mono" style={{ color: 'var(--color-text)' }}>{value}</span>
    </div>
  );
}
