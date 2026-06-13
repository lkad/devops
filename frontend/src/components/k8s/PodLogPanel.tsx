import { useEffect, useRef, useState } from 'react';
import { Modal } from '../common/Modal';
import { Button } from '../common/Button';

export interface PodLogPanelProps {
  clusterID: string;
  namespace: string;
  podName: string;
  container?: string;
}

interface LogEntry {
  ts: string;
  line: string;
}

export function PodLogPanel({ clusterID, namespace, podName, container }: PodLogPanelProps) {
  const [lines, setLines] = useState<LogEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const [filter, setFilter] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const wsRef = useRef<WebSocket | null>(null);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    const params = container ? `?container=${encodeURIComponent(container)}` : '';
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const host = window.location.host;
    const url = `${proto}://${host}/api/v1/k8s/clusters/${encodeURIComponent(clusterID)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(podName)}/logs/stream${params}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    ws.onmessage = (evt) => {
      try {
        const entry = JSON.parse(evt.data) as LogEntry;
        setLines(prev => [...prev, entry].slice(-1000));
      } catch {
        // ignore malformed
      }
    };
    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [clusterID, namespace, podName, container]);

  // Auto-scroll on new lines
  useEffect(() => {
    if (autoScroll && preRef.current) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const filtered = filter
    ? lines.filter(l => l.line.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '500px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px', borderBottom: '1px solid #333' }}>
        <span data-testid="pod-log-status" style={{ color: connected ? '#0a0' : '#a00' }}>
          {connected ? '● connected' : '● disconnected'}
        </span>
        <span style={{ fontSize: '12px', color: '#666' }}>{podName}{container ? ` / ${container}` : ''}</span>
        <input
          data-testid="pod-log-filter"
          placeholder="Filter..."
          value={filter}
          onChange={e => setFilter(e.target.value)}
          style={{ marginLeft: 'auto', padding: '4px 8px' }}
        />
        <label style={{ fontSize: '12px' }}>
          <input type="checkbox" checked={autoScroll} onChange={e => setAutoScroll(e.target.checked)} />
          {' '}Auto-scroll
        </label>
        <Button data-testid="pod-log-clear" onClick={() => setLines([])}>Clear</Button>
      </div>
      <pre
        ref={preRef}
        data-testid="pod-log-output"
        style={{
          flex: 1,
          margin: 0,
          padding: '8px',
          background: '#000',
          color: '#ddd',
          fontFamily: 'monospace',
          fontSize: '12px',
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
        }}
      >
        {filtered.map((l, i) => (
          <div key={i} data-line-ts={l.ts}>{l.line}</div>
        ))}
      </pre>
    </div>
  );
}

// Wrapper for use in K8sClusters page modal
export function PodLogModal({ open, onClose, ...props }: PodLogPanelProps & { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} title={`Logs: ${props.podName}`}>
      <PodLogPanel {...props} />
    </Modal>
  );
}
