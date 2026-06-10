// api/errors — small shared helper for surfacing API
// errors as user-facing toasts. The pattern is repeated
// across every page so a shared helper avoids drift and
// keeps the trace-ID rendering consistent.
//
// The trace ID comes from the backend's
// X-Trace-Id response header (see
// internal/observability/tracing.go). On-call paste it
// into Grafana / Tempo to find the exact trace.

import type { ApiError } from './client';

/**
 * Format an API error for a user-facing toast.
 * - Falls back to the error.message if no envelope
 * - Appends the trace ID (X-Trace-Id) when the response
 *   carried one
 * - Caps the rendered length so a long server-side
 *   stack-trace string does not blow up the toast UI
 */
export function formatApiError(prefix: string, err: ApiError | any): string {
  const message = err?.message ?? String(err);
  const trace = err?.traceId ? ` (trace: ${err.traceId})` : '';
  const body = `${prefix}: ${message}${trace}`;
  return body.length > 240 ? body.slice(0, 237) + '…' : body;
}
