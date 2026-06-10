// api/errors — small shared helper for surfacing API
// errors as user-facing toasts. The pattern is repeated
// across every page so a shared helper avoids drift and
// keeps the trace-ID rendering consistent.
//
// The trace ID comes from the backend's
// X-Trace-Id response header (see
// internal/observability/tracing.go). On-call paste it
// into Grafana / Tempo to find the exact trace — or
// click the in-toast link to land on /trace/:id which
// deep-links to Tempo when VITE_TEMPO_URL is set.

import { Link } from 'react-router-dom';
import type { ApiError } from './client';

const TOAST_MAX = 240;

/**
 * Format an API error for a user-facing toast.
 * - Falls back to the error.message if no envelope
 * - Renders the trace ID as a clickable Link to
 *   /trace/:id so the on-call can deep-dive without
 *   copy-pasting
 * - Caps the rendered length so a long server-side
 *   stack-trace string does not blow up the toast UI
 */
export function formatApiError(prefix: string, err: ApiError | any): React.ReactNode {
  const message: string = err?.message ?? String(err);
  const trace: string | undefined = err?.traceId;
  const head = `${prefix}: ${message}`;
  const body = trace
    ? `${head} · ${shortTrace(trace)}`
    : head;
  const clipped = body.length > TOAST_MAX ? body.slice(0, TOAST_MAX - 1) + '…' : body;
  if (!trace) return clipped;
  // Two children inside the toast: the message text,
  // then a small "trace" link on a new line. The
  // toast's maxWidth keeps the line wrap tidy.
  return (
    <span>
      {clipped}
      {' '}
      <Link
        to={`/trace/${trace}`}
        style={{ color: 'var(--color-primary)', textDecoration: 'underline' }}
      >
        (trace)
      </Link>
    </span>
  );
}

function shortTrace(t: string): string {
  // 32-hex trace IDs are noisy in a toast; the first
  // 8 + last 4 chars is enough for an operator to
  // spot-check two adjacent traces.
  if (t.length <= 16) return t;
  return t.slice(0, 8) + '…' + t.slice(-4);
}
