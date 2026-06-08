// Metrics — two-pane: series list (left) + sparkline detail (right).
import { useMemo, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { useApi } from '../hooks/useApi';
import { Badge } from '../components/common/Badge';
import { EmptyState } from '../components/common/EmptyState';

interface MetricSeries {
  name: string;
  target_type: string;
  target_id: string;
  labels?: Record<string, string>;
}

interface MetricPoint {
  timestamp: string;
  value: number;
  labels?: Record<string, string>;
  [k: string]: any;
}

interface SeriesResponse {
  name: string;
  points: MetricPoint[];
}

function Sparkline({ points, width = 200, height = 60 }: { points: MetricPoint[]; width?: number; height?: number }) {
  if (!points || points.length === 0) {
    return (
      <div style={{
        width, height, display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'var(--color-text-muted)', fontSize: 'var(--fs-caption)',
        border: '1px dashed var(--color-border)', borderRadius: 'var(--radius-sm)',
      }}>No data</div>
    );
  }
  const values = points.map((p) => p.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = points.length > 1 ? width / (points.length - 1) : width;
  const pad = 4;
  const polyPoints = points.map((p, i) => {
    const x = i * stepX;
    const y = height - pad - ((p.value - min) / range) * (height - 2 * pad);
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(' ');

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} style={{ display: 'block' }}>
      <rect x={0} y={0} width={width} height={height} fill="var(--color-surface-elevated)" rx={4} />
      <polyline
        points={polyPoints}
        fill="none"
        stroke="var(--color-primary)"
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function Metrics() {
  const seriesList = useApi<MetricSeries[] | { data: MetricSeries[] }>('metrics/series');
  const [selected, setSelected] = useState<{ name: string; target_type: string; target_id: string } | null>(null);

  const seriesRows: MetricSeries[] = useMemo(() => {
    const d: any = seriesList.data;
    if (!d) return [];
    return Array.isArray(d) ? d : (d.data ?? []);
  }, [seriesList.data]);

  const detail = useApi<SeriesResponse | MetricPoint[]>(
    selected ? `metrics/series/${encodeURIComponent(selected.name)}` : null,
    selected ? { target_type: selected.target_type, target_id: selected.target_id } : undefined,
  );

  const detailPoints: MetricPoint[] = useMemo(() => {
    const d: any = detail.data;
    if (!d) return [];
    if (Array.isArray(d)) return d;
    return d.points ?? [];
  }, [detail.data]);

  const last50 = detailPoints.slice(-50);

  return (
    <div>
      <PageHeader title="Metrics" subtitle="Time-series data and sparklines" />

      <div style={{ display: 'grid', gridTemplateColumns: '320px 1fr', gap: 'var(--sp-4)', minHeight: 480 }}>
        {/* Left: series list */}
        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          overflow: 'hidden',
          display: 'flex', flexDirection: 'column',
        }}>
          <div style={{
            padding: 'var(--sp-3) var(--sp-4)',
            borderBottom: '1px solid var(--color-border)',
            fontSize: 'var(--fs-caption)',
            textTransform: 'uppercase',
            letterSpacing: 0.5,
            fontWeight: 600,
            color: 'var(--color-text-secondary)',
          }}>Series ({seriesRows.length})</div>
          <div style={{ overflow: 'auto', flex: 1 }}>
            {seriesList.loading && <div style={{ padding: 'var(--sp-4)', color: 'var(--color-text-muted)' }}>Loading…</div>}
            {seriesList.error && <div style={{ padding: 'var(--sp-4)', color: 'var(--color-error)' }}>{seriesList.error}</div>}
            {!seriesList.loading && seriesRows.length === 0 && (
              <div style={{ padding: 'var(--sp-4)' }}>
                <EmptyState title="No series" hint="No metric series have been ingested yet." />
              </div>
            )}
            {seriesRows.map((s) => {
              const isActive = selected?.name === s.name
                && selected?.target_type === s.target_type
                && selected?.target_id === s.target_id;
              return (
                <button
                  key={`${s.name}|${s.target_type}|${s.target_id}`}
                  onClick={() => setSelected({ name: s.name, target_type: s.target_type, target_id: s.target_id })}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    background: isActive ? 'var(--color-primary-muted)' : 'transparent',
                    border: 'none',
                    borderLeft: isActive ? '2px solid var(--color-primary)' : '2px solid transparent',
                    borderBottom: '1px solid var(--color-border-subtle)',
                    padding: 'var(--sp-3) var(--sp-4)',
                    cursor: 'pointer',
                    color: 'var(--color-text)',
                    display: 'flex', flexDirection: 'column', gap: 2,
                  }}
                >
                  <span className="mono" style={{ fontSize: 'var(--fs-small)', fontWeight: 500 }}>{s.name}</span>
                  <span style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }}>
                    {s.target_type} <span style={{ opacity: 0.5 }}>·</span> <span className="mono">{s.target_id}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        {/* Right: detail */}
        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-5)',
        }}>
          {!selected && (
            <EmptyState title="Select a series" hint="Pick a metric on the left to view its sparkline." />
          )}
          {selected && (
            <div>
              <div style={{ marginBottom: 'var(--sp-4)' }}>
                <h2 style={{ fontSize: 'var(--fs-h3)' }} className="mono">{selected.name}</h2>
                <div style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--fs-caption)', marginTop: 4 }}>
                  {selected.target_type} · <span className="mono">{selected.target_id}</span>
                </div>
              </div>

              {detail.loading && <div style={{ color: 'var(--color-text-muted)' }}>Loading…</div>}
              {detail.error && <div style={{ color: 'var(--color-error)' }}>{detail.error}</div>}

              {!detail.loading && !detail.error && (
                <>
                  <div style={{ marginBottom: 'var(--sp-5)' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--sp-2)' }}>
                      <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600 }}>
                        Last 50 points
                      </div>
                      <Badge tone="neutral">{detailPoints.length} total</Badge>
                    </div>
                    <Sparkline points={last50} />
                  </div>

                  <div>
                    <div style={{ fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600, marginBottom: 'var(--sp-2)' }}>
                      Recent values
                    </div>
                    {last50.length === 0 ? (
                      <div style={{ color: 'var(--color-text-muted)' }}>No points recorded.</div>
                    ) : (
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 0, maxHeight: 280, overflow: 'auto', border: '1px solid var(--color-border-subtle)', borderRadius: 'var(--radius-sm)' }}>
                        {[...last50].reverse().map((p, i) => (
                          <div key={i} style={{ display: 'contents' }}>
                            <div style={{ padding: '6px 12px', borderBottom: '1px solid var(--color-border-subtle)', fontSize: 'var(--fs-caption)', color: 'var(--color-text-secondary)' }} className="mono">
                              {p.timestamp}
                            </div>
                            <div style={{ padding: '6px 12px', borderBottom: '1px solid var(--color-border-subtle)', fontSize: 'var(--fs-caption)', textAlign: 'right' }} className="mono">
                              {typeof p.value === 'number' ? p.value.toFixed(3) : String(p.value)}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
