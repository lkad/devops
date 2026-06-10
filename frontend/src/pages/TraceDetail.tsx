// TraceDetail — single trace lookup page.
//
// Opened from an error toast's trace id. The page
// shows the trace id prominently and, when a Tempo
// URL is configured, links to the matching Tempo
// trace view. When no Tempo is configured, the page
// shows the trace id in a copyable <pre> block so
// the on-call can paste it into Grafana / Jaeger
// / wherever their stack lives.

// eslint-disable-next-line @typescript-eslint/triple-slash-reference
/// <reference types="vite/client" />

import { useParams } from 'react-router-dom';
import { PageHeader } from '../components/common/PageHeader';
import { Button } from '../components/common/Button';

const TEMPO_URL = (import.meta.env.VITE_TEMPO_URL as string | undefined) ?? '';

function buildTempoURL(traceId: string, base: string): string | null {
  if (!base) return null;
  // Tempo's Grafana datasource URL format:
  //   <grafana>/explore?left=...
  // We use the more permissive TraceQL search URL:
  //   <tempo>/api/traces/<traceId>
  // Operators can override by setting the env to the
  // exact template they use (a {traceId} placeholder
  // is supported).
  if (base.includes('{traceId}')) {
    return base.replace('{traceId}', traceId);
  }
  return base.replace(/\/+$/, '') + '/api/traces/' + traceId;
}

export function TraceDetail() {
  const { id } = useParams<{ id: string }>();
  const traceId = id ?? '';
  const tempoURL = buildTempoURL(traceId, TEMPO_URL);

  async function copyToClipboard() {
    try {
      await navigator.clipboard.writeText(traceId);
    } catch {
      // Clipboard API is best-effort. The <pre>
      // below is also selectable.
    }
  }

  return (
    <div>
      <PageHeader
        title="Trace"
        subtitle="OpenTelemetry trace id from the failed request"
      />

      <div
        style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--sp-5)',
          maxWidth: 720,
        }}
      >
        <div
          style={{
            fontSize: 'var(--fs-caption)',
            color: 'var(--color-text-secondary)',
            marginBottom: 'var(--sp-2)',
          }}
        >
          Trace id
        </div>
        <pre
          data-testid="trace-id"
          style={{
            margin: 0,
            padding: 'var(--sp-3) var(--sp-4)',
            background: 'var(--color-surface-elevated)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-sm)',
            fontFamily: 'var(--font-mono, monospace)',
            fontSize: 'var(--fs-mono)',
            overflowX: 'auto',
            userSelect: 'all',
          }}
        >
          {traceId}
        </pre>

        <div
          style={{
            display: 'flex',
            gap: 'var(--sp-3)',
            marginTop: 'var(--sp-4)',
            alignItems: 'center',
          }}
        >
          <Button onClick={copyToClipboard}>Copy</Button>
          {tempoURL && (
            <a
              href={tempoURL}
              target="_blank"
              rel="noreferrer"
              data-testid="tempo-link"
            >
              <Button variant="secondary">Open in Tempo ↗</Button>
            </a>
          )}
        </div>

        {!tempoURL && (
          <div
            style={{
              marginTop: 'var(--sp-4)',
              padding: 'var(--sp-3) var(--sp-4)',
              border: '1px dashed var(--color-border)',
              borderRadius: 'var(--radius-sm)',
              color: 'var(--color-text-muted)',
              fontSize: 'var(--fs-small)',
            }}
          >
            No Tempo URL configured. Set{' '}
            <code style={{ fontFamily: 'var(--font-mono, monospace)' }}>
              VITE_TEMPO_URL
            </code>{' '}
            in <code>.env</code> (supports a{' '}
            <code>{'{traceId}'}</code> placeholder) and reload
            to enable the in-app deep link.
          </div>
        )}
      </div>
    </div>
  );
}
