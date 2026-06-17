// TraceDetail — single trace lookup page.
//
// Opened from an error toast's trace id. The page
// shows the trace id prominently and, when a Tempo
// URL is configured, links to the matching Tempo
// trace view. When no Tempo is configured, the page
// shows the trace id in a copyable <pre> block so
// the on-call can paste it into Grafana / Jaeger
// / wherever their stack lives. The page also shows
// when the trace was opened and offers a deep link
// to the Logs page (where the trace id is one of the
// existing search fields).

// eslint-disable-next-line @typescript-eslint/triple-slash-reference
/// <reference types="vite/client" />

import { useTranslation } from 'react-i18next';
import { useParams, Link } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { PageHeader } from '../components/common/PageHeader';
import { Button } from '../components/common/Button';

const TEMPO_URL = (import.meta.env.VITE_TEMPO_URL as string | undefined) ?? '';

function buildTempoURL(traceId: string, base: string): string | null {
  if (!base) return null;
  // Tempo's TraceQL search URL:
  //   <tempo>/api/traces/<traceId>
  // Operators can override by setting the env to the
  // exact template they use (a {traceId} placeholder
  // is supported).
  if (base.includes('{traceId}')) {
    return base.replace('{traceId}', traceId);
  }
  return base.replace(/\/+$/, '') + '/api/traces/' + traceId;
}

function fmtTimestamp(d: Date, locale: string): string {
  // Intl.DateTimeFormat picks the locale's date+time format
  // (YYYY-MM-DD HH:mm in zh-CN; M/D/YYYY, h:mm AM in en-US;
  // DD/MM/YYYY HH:mm in most of Europe). Falls back to the
  // default formatter for any locale tag Intl doesn't recognize.
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(d);
}

export function TraceDetail() {
  const { t, i18n } = useTranslation('trace');
  const { id } = useParams<{ id: string }>();
  const traceId = id ?? '';
  const tempoURL = buildTempoURL(traceId, TEMPO_URL);
  const [openedAt] = useState(() => new Date());

  // Refresh "how long ago" every 30s so the page is
  // useful even when the operator has it open in a
  // background tab.
  const [, setTick] = useState(0);
  useEffect(() => {
    const t = window.setInterval(() => setTick((n) => n + 1), 30_000);
    return () => window.clearInterval(t);
  }, []);

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
        title={t('title')}
        subtitle={t('subtitle')}
        actions={
          <Link to="/services">
            <Button variant="secondary">{t('actions.back-to-services')}</Button>
          </Link>
        }
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
            display: 'grid',
            gridTemplateColumns: '120px 1fr',
            gap: 'var(--sp-2) var(--sp-4)',
            marginBottom: 'var(--sp-4)',
            fontSize: 'var(--fs-small)',
          }}
        >
          <div style={{ color: 'var(--color-text-secondary)' }}>{t('fields.trace-id')}</div>
          <div
            className="mono"
            style={{ wordBreak: 'break-all' }}
            data-testid="trace-id"
          >
            {traceId}
          </div>

          <div style={{ color: 'var(--color-text-secondary)' }}>{t('fields.opened')}</div>
          <div>{fmtTimestamp(openedAt, i18n.language)}</div>

          <div style={{ color: 'var(--color-text-secondary)' }}>{t('fields.source')}</div>
          <div>
            <code style={{ fontSize: 'var(--fs-caption)' }}>X-Trace-Id</code>
            {t('fields.source-detail')}
          </div>
        </div>

        <pre
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
            flexWrap: 'wrap',
          }}
        >
          <Button onClick={copyToClipboard}>{t('actions.copy')}</Button>
          {tempoURL && (
            <a
              href={tempoURL}
              target="_blank"
              rel="noreferrer"
              data-testid="tempo-link"
            >
              <Button variant="secondary">{t('actions.open-in-tempo')}</Button>
            </a>
          )}
          <Link
            to={`/logs?q=${encodeURIComponent(traceId)}`}
            data-testid="logs-link"
          >
            <Button variant="secondary">{t('actions.search-logs')}</Button>
          </Link>
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
            {t('tempo-not-configured', {
              env: 'VITE_TEMPO_URL',
              file: '.env',
              placeholder: '{traceId}',
            })}
          </div>
        )}
      </div>
    </div>
  );
}
