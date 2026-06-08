// useWebSocket — connects to the backend /api/v1/ws hub.
// Channel-based subscription: callers pass channel names; the
// hook returns the latest event per channel. Reconnect is
// automatic with exponential backoff (capped at 5 retries).

import { useEffect, useRef, useState, useCallback } from 'react';
import { getToken } from '../api/client';

export interface WsEvent {
  channel: string;
  type: string;
  data: any;
  timestamp: string;
  client_id?: string;
}

type Handler = (e: WsEvent) => void;

export function useWebSocket(channels: string[] = [], onEvent?: Handler) {
  const [connected, setConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<WsEvent | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const retryRef = useRef(0);
  const channelsRef = useRef(channels);
  const onEventRef = useRef(onEvent);
  channelsRef.current = channels;
  onEventRef.current = onEvent;

  const connect = useCallback(() => {
    const tok = getToken();
    if (!tok) return;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/api/v1/ws?token=${encodeURIComponent(tok)}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      retryRef.current = 0;
      for (const ch of channelsRef.current) {
        ws.send(JSON.stringify({ type: 'subscribe', channel: ch }));
      }
    };

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as WsEvent;
        setLastEvent(msg);
        onEventRef.current?.(msg);
      } catch { /* ignore */ }
    };

    ws.onerror = () => { /* close fires next */ };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      if (retryRef.current < 5) {
        const delay = Math.min(1000 * 2 ** retryRef.current, 30000);
        retryRef.current++;
        setTimeout(connect, delay);
      }
    };
  }, []);

  useEffect(() => {
    connect();
    return () => {
      retryRef.current = 5; // stop reconnecting on unmount
      wsRef.current?.close();
    };
  }, [connect]);

  // Subscribe to new channels when the prop changes.
  useEffect(() => {
    if (!connected || !wsRef.current) return;
    for (const ch of channels) {
      wsRef.current.send(JSON.stringify({ type: 'subscribe', channel: ch }));
    }
  }, [channels, connected]);

  return { connected, lastEvent };
}
