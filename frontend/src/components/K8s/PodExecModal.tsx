// PodExecModal — in-platform shell into a running pod.
//
// Flow:
//   1. POST /exec → backend returns { sessionId }.
//   2. Open WS to /exec/:sessionId/ws.
//   3. Pipe xterm input → WS frames; pipe WS frames → xterm.write.
//   4. On xterm resize, send a window-size frame.
//   5. On WS drop: try to auto-reconnect up to MAX_RETRIES with
//      exponential backoff. If retries exhausted, write a banner
//      and stop.
//
// The protocol mirrors the kubectl exec pattern: a single WS per
// session carrying binary or text frames. We send JSON envelopes so
// the same socket can carry resize commands and stdin bytes.

import { useEffect, useRef, useState, useCallback } from 'react';
import { Button } from '../common/Button';
import { apiPost, getToken } from '../../api/client';

// CSS must be imported once for xterm to render correctly.
import '@xterm/xterm/css/xterm.css';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';

const MAX_RETRIES = 3;
const RETRY_BASE_MS = 500;

export interface PodExecModalProps {
  open: boolean;
  onClose: () => void;
  clusterId: string;
  namespace: string;
  pod: string;
  container?: string;
  command?: string[];
}

function buildWsUrl(path: string, token: string | null): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${proto}//${location.host}${path}${q}`;
}

export function PodExecModal({
  open,
  onClose,
  clusterId,
  namespace,
  pod,
  container,
  command,
}: PodExecModalProps) {
  const termRef = useRef<HTMLDivElement | null>(null);
  // We keep the Terminal instance outside React state — it has
  // its own render lifecycle and writing to it from effects is
  // what we want, not re-renders.
  const termInstanceRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const retriesRef = useRef(0);
  const sessionIdRef = useRef<string | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const commandRef = useRef<string[] | undefined>(command);
  commandRef.current = command;

  const [status, setStatus] = useState<'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed' | 'error'>('idle');
  const [error, setError] = useState<string | null>(null);

  const writeBanner = useCallback((text: string) => {
    termInstanceRef.current?.writeln(`\r\n\x1b[33m${text}\x1b[0m`);
  }, []);

  const teardown = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    try { wsRef.current?.close(); } catch { /* noop */ }
    wsRef.current = null;
  }, []);

  // Open the WS given a sessionId. Used by the initial connect
  // and the retry path.
  const openSocket = useCallback((sessionId: string) => {
    const tok = getToken();
    const path = `/api/v1/k8s/clusters/${encodeURIComponent(clusterId)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/exec/${encodeURIComponent(sessionId)}/ws`;
    const ws = new WebSocket(buildWsUrl(path, tok));
    wsRef.current = ws;

    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      setStatus('open');
      setError(null);
      retriesRef.current = 0;
      // Send initial resize.
      const t = termInstanceRef.current;
      if (t) {
        ws.send(JSON.stringify({ type: 'resize', cols: t.cols, rows: t.rows }));
      }
    };

    ws.onmessage = (ev) => {
      // Backend may send text frames (xterm.write) or JSON
      // envelopes ({ type: 'exit', code }).
      if (typeof ev.data === 'string') {
        try {
          const j = JSON.parse(ev.data);
          if (j && typeof j === 'object') {
            if (j.type === 'exit') {
              termInstanceRef.current?.writeln(`\r\n\x1b[33m[session ended, code=${j.code ?? '?'}]\x1b[0m`);
              setStatus('closed');
              try { ws.close(); } catch { /* noop */ }
              return;
            }
            if (typeof j.data === 'string') {
              termInstanceRef.current?.write(j.data);
              return;
            }
          }
        } catch { /* not JSON — treat as raw output */ }
        termInstanceRef.current?.write(ev.data);
      } else if (ev.data instanceof ArrayBuffer) {
        const text = new TextDecoder().decode(new Uint8Array(ev.data));
        termInstanceRef.current?.write(text);
      }
    };

    ws.onerror = () => { /* onclose will follow */ };

    ws.onclose = () => {
      wsRef.current = null;
      if (statusRef.current === 'closed' || statusRef.current === 'idle') return;
      if (retriesRef.current < MAX_RETRIES) {
        const attempt = retriesRef.current;
        retriesRef.current += 1;
        setStatus('reconnecting');
        writeBanner(`reconnecting… (${attempt + 1}/${MAX_RETRIES})`);
        const delay = RETRY_BASE_MS * Math.pow(2, attempt);
        reconnectTimerRef.current = window.setTimeout(() => {
          reconnectTimerRef.current = null;
          openSocket(sessionId);
        }, delay);
      } else {
        setStatus('error');
        writeBanner('connection lost — retries exhausted');
      }
    };
  }, [clusterId, namespace, pod, writeBanner]);

  // Mirror status into a ref so the onclose handler reads the
  // latest value without capturing a stale closure.
  const statusRef = useRef(status);
  statusRef.current = status;

  // Effect: mount xterm + open exec session on open.
  useEffect(() => {
    if (!open) return;

    setError(null);
    setStatus('connecting');
    retriesRef.current = 0;

    // Mount terminal.
    let term: Terminal | null = null;
    let fit: FitAddon | null = null;
    if (termRef.current && !termInstanceRef.current) {
      term = new Terminal({
        cursorBlink: true,
        convertEol: true,
        fontFamily: 'var(--font-mono), monospace',
        fontSize: 13,
        theme: {
          background: '#0c1220',
          foreground: '#f1f5f9',
          cursor: '#22d3ee',
        },
      });
      fit = new FitAddon();
      term.loadAddon(fit);
      term.open(termRef.current);
      try { fit.fit(); } catch { /* container not yet measured */ }
      termInstanceRef.current = term;
      fitRef.current = fit;
      term.writeln('\x1b[36mconnecting to pod shell…\x1b[0m');

      term.onData((data) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'stdin', data }));
        }
      });
      term.onResize(({ cols, rows }) => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'resize', cols, rows }));
        }
        try { fit?.fit(); } catch { /* noop */ }
      });
    }

    // Resize observer keeps the terminal matching the modal.
    const ro = new ResizeObserver(() => {
      try { fitRef.current?.fit(); } catch { /* noop */ }
    });
    if (termRef.current) ro.observe(termRef.current);

    // 1) create exec session
    const ac = new AbortController();
    const body: Record<string, any> = {};
    if (container) body.container = container;
    if (command && command.length) body.command = command;

    apiPost<{ sessionId?: string; data?: { sessionId?: string } }>(
      `k8s/clusters/${encodeURIComponent(clusterId)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/exec`,
      body,
    )
      .then((resp) => {
        const sid = resp?.sessionId ?? resp?.data?.sessionId;
        if (!sid) throw new Error('no session id returned');
        sessionIdRef.current = sid;
        openSocket(sid);
      })
      .catch((e: any) => {
        if (e?.name === 'AbortError') return;
        setStatus('error');
        setError(e?.message ?? 'failed to start exec session');
        writeBanner(`error: ${e?.message ?? 'failed to start exec'}`);
      });

    return () => {
      ac.abort();
      ro.disconnect();
      teardown();
      // Dispose terminal cleanly so re-opening gives a fresh one.
      try { termInstanceRef.current?.dispose(); } catch { /* noop */ }
      termInstanceRef.current = null;
      fitRef.current = null;
      sessionIdRef.current = null;
      setStatus('idle');
    };
  }, [open, clusterId, namespace, pod, container, openSocket, teardown, writeBanner]);

  if (!open) return null;

  const title = `Shell: ${pod}${namespace ? ` (${namespace})` : ''}`;

  return (
    <div
      data-testid="pod-exec-modal"
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
          width: 960,
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
              data-testid="pod-exec-status"
              style={{
                fontSize: 'var(--fs-caption)',
                color:
                  status === 'open' ? 'var(--color-success)' :
                  status === 'reconnecting' ? 'var(--color-warning)' :
                  status === 'error' ? 'var(--color-error)' :
                  'var(--color-text-muted)',
                textTransform: 'uppercase',
                letterSpacing: 0.5,
              }}
            >
              {status}
            </span>
            <Button variant="ghost" onClick={onClose}><span data-testid="pod-exec-close">×</span></Button>
          </div>
        </div>
        {error && (
          <div style={{ padding: 'var(--sp-2) var(--sp-5)', color: 'var(--color-error)', fontSize: 'var(--fs-small)' }}>
            {error}
          </div>
        )}
        <div
          ref={termRef}
          data-testid="pod-exec-terminal"
          style={{
            padding: 'var(--sp-2)',
            background: 'var(--color-bg)',
            flex: 1,
            minHeight: 360,
            maxHeight: 'calc(85vh - 110px)',
            overflow: 'hidden',
          }}
        />
      </div>
    </div>
  );
}
