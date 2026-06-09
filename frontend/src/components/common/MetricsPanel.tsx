// MetricsPanel — small data-bound panel that fetches
// /api/v1/physical-hosts/:id/metrics and renders CPU / memory /
// disk / uptime. Status badge is driven by data_status so the
// operator can tell fresh / stale / unavailable apart without
// reading the JSON.
//
// The panel is intentionally chart-free: a CSS-bar style keeps
// the dependency surface flat. A future iteration can plug in
// a chart library; the API and the data shape stay the same.
import { useEffect, useState } from 'react';
import { apiGet } from '../../api/client';
import { Badge, toneForDataStatus } from './Badge';

export interface HostMetrics {
  cpu: { cores: number; usage_percent: number };
  memory: { total_mib: number; used_mib: number; usage_percent: number };
  disk: { disks: { mount: string; size_gb: number; used_gb: number; usage_percent: number }[] };
  uptime: { seconds: number; formatted: string };
  data_status: 'fresh' | 'stale' | 'unavailable' | string;
  collected_at?: string;
  warnings?: string[];
}

function fmtBytesMiB(mib: number): string {
  if (mib < 1024) return `${mib} MiB`;
  return `${(mib / 1024).toFixed(1)} GiB`;
}

function fmtUptime(s: number): string {
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  return d > 0 ? `${d}d ${h}h ${m}m` : h > 0 ? `${h}h ${m}m` : `${m}m`;
}

// Bar renders a 0-100% bar with the standard track + fill
// palette. The width is clamped to [0, 100] so a pathological
// reading above 100% (e.g. cache pressure overshoot) does not
// break the layout.
function Bar({ percent, fill }: { percent: number; fill: string }) {
  const w = Math.max(0, Math.min(100, percent));
  return (
    <div
      style={{
        width: '100%',
        height: 6,
        background: 'var(--color-surface-elevated)',
        borderRadius: 3,
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          width: `${w}%`,
          height: '100%',
          background: fill,
          transition: 'width 200ms ease-out',
        }}
      />
    </div>
  );
}

export function MetricsPanel({ hostId, refreshKey = 0 }: { hostId: string; refreshKey?: number }) {
  const [data, setData] = useState<HostMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    apiGet<HostMetrics>(`physical-hosts/${hostId}/metrics`)
      .then((d) => { if (!cancelled) { setData(d); setLoading(false); } })
      .catch((e) => { if (!cancelled) { setError(e?.message ?? 'failed'); setLoading(false); } });
    return () => { cancelled = true; };
    // refreshKey lets the parent force a re-fetch (e.g. on
    // device_event arrival) without rebuilding the component.
  }, [hostId, refreshKey]);

  if (loading) {
    return <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-small)' }}>Loading metrics…</div>;
  }
  if (error || !data) {
    return <div style={{ color: 'var(--color-error)', fontSize: 'var(--fs-small)' }}>Metrics: {error ?? 'unavailable'}</div>;
  }

  const tone = toneForDataStatus(data.data_status);

  return (
    <div
      style={{
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--sp-3) var(--sp-4)',
        marginTop: 'var(--sp-3)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 'var(--sp-2)' }}>
        <span style={{ fontSize: 'var(--fs-caption)', textTransform: 'uppercase', color: 'var(--color-text-muted)' }}>Metrics</span>
        <Badge tone={tone}>{data.data_status}</Badge>
      </div>

      {data.data_status === 'unavailable' && (
        <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-small)', marginBottom: 'var(--sp-2)' }}>
          No data available. Last probe failed.
        </div>
      )}

      {/* CPU */}
      <div style={{ marginBottom: 'var(--sp-2)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-small)' }}>
          <span>CPU</span>
          <span style={{ color: 'var(--color-text-muted)' }}>{data.cpu.usage_percent.toFixed(1)}% / {data.cpu.cores} cores</span>
        </div>
        <Bar percent={data.cpu.usage_percent} fill="var(--color-info)" />
      </div>

      {/* Memory */}
      <div style={{ marginBottom: 'var(--sp-2)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-small)' }}>
          <span>Memory</span>
          <span style={{ color: 'var(--color-text-muted)' }}>{fmtBytesMiB(data.memory.used_mib)} / {fmtBytesMiB(data.memory.total_mib)} ({data.memory.usage_percent.toFixed(1)}%)</span>
        </div>
        <Bar percent={data.memory.usage_percent} fill="var(--color-warning)" />
      </div>

      {/* Disk */}
      {data.disk.disks.length === 0 ? (
        <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-small)' }}>No disks reported</div>
      ) : (
        data.disk.disks.map((d, i) => (
          <div key={`${d.mount}-${i}`} style={{ marginBottom: 'var(--sp-2)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-small)' }}>
              <span>Disk <code style={{ color: 'var(--color-text-muted)' }}>{d.mount}</code></span>
              <span style={{ color: 'var(--color-text-muted)' }}>{d.used_gb}GB / {d.size_gb}GB ({d.usage_percent.toFixed(1)}%)</span>
            </div>
            <Bar percent={d.usage_percent} fill="var(--color-success)" />
          </div>
        ))
      )}

      {/* Uptime */}
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-small)', color: 'var(--color-text-muted)', marginTop: 'var(--sp-2)' }}>
        <span>Uptime</span>
        <span>{fmtUptime(data.uptime.seconds)}</span>
      </div>

      {data.warnings && data.warnings.length > 0 && (
        <details style={{ marginTop: 'var(--sp-2)' }}>
          <summary style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)', cursor: 'pointer' }}>{data.warnings.length} warning(s)</summary>
          <ul style={{ margin: 'var(--sp-1) 0 0 var(--sp-4)', padding: 0, color: 'var(--color-text-muted)', fontSize: 'var(--fs-caption)' }}>
            {data.warnings.map((w, i) => (<li key={i}>{w}</li>))}
          </ul>
        </details>
      )}
    </div>
  );
}
