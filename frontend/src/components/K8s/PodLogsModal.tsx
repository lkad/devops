// PodLogsModal — live log viewer for a single pod.
//
// Strategy:
//   1. On open: fetch the one-shot /logs endpoint (tail 100 lines)
//      and render them as the initial buffer. The on-call engineer
//      sees the recent past immediately, even before the stream
//      catches up.
//   2. Then open the append-only WebSocket stream (/logs/stream).
//      If WS fails (some intermediaries block it), fall back to
//      EventSource on /logs/sse.
//   3. Buffer is capped at MAX_LINES; oldest are dropped silently
//      to keep the DOM responsive for 10k+ line bursts. The cap
//      (not react-window) was chosen because log lines are short
//      and uniform — virtualization adds DOM-measurement cost that
//      defeats the purpose on a fast stream.
//   4. Auto-scroll to bottom by default; user can pause to read.
//
// Buffer cap rationale (over react-window): xterm-style virtualization
// needs measured row heights and a scrollable container; the per-line
// overhead for short log lines is dominated by DOM nodes, not layout.
// A bounded array + capped render is simpler and just as fast.

import { useEffect, useRef, useState, useCallback } from 'react';
import { Button } from '../common/Button';

const MAX_LINES = 5000;
const INITIAL_LINES = 100;

export interface PodLogsModalProps {
  open: boolean;
  onClose: () => void;
  clusterId: string;
  namespace: string;
  pod: string;
  container?: string;
  /** Optional bearer token; defaults to localStorage token. */
  getToken?: () => string | null;
}

interface LogResponse {
  data?: { lines?: string[]; timestamps?: string[] };
  lines?: string[];
  timestamps?: string[];
}

function buildWsUrl(path: string, token: string | null): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${proto}//${location.host}${path}${q}`;
}

function buildSseUrl(path: string, token: string | null): string {
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${path}${q}`;
}

function tokenFromStorage(): string | null {
  try { return localStorage.getItem('devops-toolkit.jwt'); } catch { return null; }
}

export function PodLogsModal({
  open,
  onClose,
  clusterId,
  namespace,
  pod,
  container,
  getToken = tokenFromStorage,
}: PodLogsModalProps) {
  const [lines, setLines] = useState<string[]>([]);
  const [paused, setPaused] = useState(false);
  const [transport, setTransport] = useState<'ws' | 'sse' | 'none' | 'connecting'>('none');
  const [error, setError] = useState<string | null>(null);
  const [disconnected, setDisconnected] = useState(false);

  const containerRef = useRef<HTMLDivElement | null>(null);
  const pausedRef = useRef(paused);
  const wasAtBottomRef = useRef(true);
  pausedRef.current = paused;

  const appendLines = useCallback((incoming: string[]) => {
    if (incoming.length === 0) return;
    setLines((prev) => {
      const next = prev.length + incoming.length > MAX_LINES
        ? prev.slice(prev.length + incoming.length - MAX_LINES).concat(incoming)
        : prev.concat(incoming);
      return next;
    });
  }, []);

  // Auto-scroll to bottom on new lines unless user has scrolled up.
  useEffect(() => {
    const el = containerRef.current;
    if (!el || pausedRef.current) return;
    if (wasAtBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [lines]);

  function onScroll() {
    const el = containerRef.current;
    if (!el) return;
    // 8px threshold to avoid jitter at the very bottom.
    wasAtBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 8;
  }

  // Lifecycle: open one-shot, then stream.
  useEffect(() => {
    if (!open) return;
    setLines([]);
    setError(null);
    setDisconnected(false);
    setTransport('connecting');

    const tok = getToken();
    const params = new URLSearchParams();
    params.set('tail', String(INITIAL_LINES));
    if (container) params.set('container', container);
    const oneShotUrl = `api/v1/k8s/clusters/${encodeURIComponent(clusterId)}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/logs?${params.toString()}`;

    const ac = new AbortController();
    let cancelled = false;

    // 1) one-shot
    fetch(oneShotUrl, {
      headers: tok ? { Authorization: `Bearer ${tok}` } : {},
      signal: ac.signal,
    })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<LogResponse>;
      })
      .then((data) => {
        if (cancelled) return;
        const initial = data?.data?.lines ?? data?.lines ?? [];
        if (initial.length) appendLines(initial);
      })
      .catch((e) => {
        if (cancelled) return;
        if (e?.name === 'AbortError') return;
        setError(`fetch logs: ${e?.message ?? 'failed'}`);
      });

    // 2) live stream — try WebSocket first, fall back to SSE.
    const wsPath = `/api/v1/k8s/clusters/${encodeURIComponent(clusterId)}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/logs/stream${container ? `?container=${encodeURIComponent(container)}` : ''}`;
    let ws: WebSocket | null = null;
    let es: EventSource | null = null;
    let didFallback = false;

    try {
      ws = new WebSocket(buildWsUrl(wsPath, tok));
      ws.onopen = () => {
        if (cancelled) return;
        setTransport('ws');
      };
      ws.onmessage = (ev) => {
        // Expect either a single line or a JSON {lines: [...]} frame.
        const raw = typeof ev.data === 'string' ? ev.data : '';
        if (!raw) return;
        let parsed: string[] | null = null;
        try {
          const j = JSON.parse(raw);
          if (Array.isArray(j?.lines)) parsed = j.lines;
          else if (typeof j?.line === 'string') parsed = [j.line];
        } catch {
          // raw may be a single line terminated by \n
          parsed = raw.split('\n').filter((s) => s.length > 0);
        }
        if (parsed) appendLines(parsed);
      };
      ws.onerror = () => { /* let onclose drive fallback */ };
      ws.onclose = () => {
        if (cancelled || didFallback) return;
        didFallback = true;
        // Try SSE.
        try {
          es = new EventSource(buildSseUrl(wsPath.replace('/stream', '/sse'), tok));
          es.onopen = () => setTransport('sse');
          es.onerror = () => {
            if (cancelled) return;
            setDisconnected(true);
            setTransport('none');
          };
          es.onmessage = (ev) => {
            const raw = (ev as MessageEvent).data ?? '';
            if (!raw) return;
            let parsed: string[] | null = null;
            try {
              const j = JSON.parse(raw);
              if (Array.isArray(j?.lines)) parsed = j.lines;
              else if (typeof j?.line === 'string') parsed = [j.line];
            } catch {
              parsed = raw.split('\n').filter((s: string) => s.length > 0);
            }
            if (parsed) appendLines(parsed);
          };
        } catch (e) {
          setDisconnected(true);
          setTransport('none');
        }
      };
    } catch {
      // Construction failed — go straight to SSE.
      didFallback = true;
      try {
        es = new EventSource(buildSseUrl(wsPath.replace('/stream', '/sse'), tok));
        es.onopen = () => setTransport('sse');
        es.onmessage = (ev) => {
          const raw = (ev as MessageEvent).data ?? '';
          if (!raw) return;
          appendLines(raw.split('\n').filter((s: string) => s.length > 0));
        };
        es.onerror = () => {
          if (cancelled) return;
          setDisconnected(true);
          setTransport('none');
        };
      } catch {
        setDisconnected(true);
        setTransport('none');
      }
    }

    return () => {
      cancelled = true;
      ac.abort();
      try { ws?.close(); } catch { /* noop */ }
      try { es?.close(); } catch { /* noop */ }
    };
  }, [open, clusterId, namespace, pod, container, getToken, appendLines]);

  // Renderless when closed (the page also checks `open`).
  if (!open) return null;

  const title = `Logs: ${pod}${namespace ? ` (${namespace})` : ''}`;

  return (
    <div
      data-testid="pod-logs-modal"
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-lg)',
          width: 880,
          maxWidth: '95vw',
          maxHeight: '85vh',
          display: 'flex', flexDirection: 'column',
        }}
      >
        <div
          style={{
            padding: 'var(--sp-3) var(--sp-5)',
            borderBottom: '1px solid var(--color-border)',
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            gap: 'var(--sp-3)',
          }}
        >
          <h3 style={{ fontSize: 'var(--fs-h3)', margin: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</h3>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
            <span
              data-testid="pod-logs-transport"
              style={{
                fontSize: 'var(--fs-caption)',
                color: transport === 'ws' || transport === 'sse' ? 'var(--color-success)' : disconnected ? 'var(--color-error)' : 'var(--color-text-muted)',
                textTransform: 'uppercase',
                letterSpacing: 0.5,
              }}
            >
              {transport === 'connecting' ? 'connecting…' : transport === 'none' && disconnected ? 'disconnected' : transport}
            </span>
            <Button
              variant="secondary"
              onClick={() => setPaused((p) => !p)}
            >
              <span data-testid="pod-logs-pause">{paused ? 'Resume' : 'Pause'}</span>
            </Button>
            <Button variant="ghost" onClick={onClose}><span data-testid="pod-logs-close">×</span></Button>
          </div>
        </div>
        {error && (
          <div style={{ padding: 'var(--sp-2) var(--sp-5)', color: 'var(--color-error)', fontSize: 'var(--fs-small)' }}>
            {error}
          </div>
        )}
        <div
          ref={containerRef}
          onScroll={onScroll}
          data-testid="pod-logs-body"
          style={{
            padding: 'var(--sp-3) var(--sp-4)',
            overflow: 'auto',
            background: 'var(--color-bg)',
            color: 'var(--color-text)',
            fontFamily: 'var(--font-mono)',
            fontSize: 'var(--fs-mono)',
            flex: 1,
            minHeight: 320,
            maxHeight: 'calc(85vh - 120px)',
            whiteSpace: 'pre',
            lineHeight: 1.4,
          }}
        >
          {lines.length === 0 ? (
            <div style={{ color: 'var(--color-text-muted)' }}>Waiting for logs…</div>
          ) : (
            lines.map((l, i) => <div key={i} data-testid="pod-logs-line">{l}</div>)
          )}
        </div>
        <div
          style={{
            padding: 'var(--sp-2) var(--sp-5)',
            borderTop: '1px solid var(--color-border)',
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            fontSize: 'var(--fs-caption)', color: 'var(--color-text-muted)',
          }}
        >
          <span>{lines.length} line{lines.length === 1 ? '' : 's'} (cap {MAX_LINES})</span>
          <span>{paused ? 'scroll paused' : 'live'}</span>
        </div>
      </div>
    </div>
  );
}
