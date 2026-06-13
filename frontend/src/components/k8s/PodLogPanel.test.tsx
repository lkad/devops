import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { PodLogPanel } from './PodLogPanel';

// Mock WebSocket
class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  readyState: number = 0; // CONNECTING
  onopen: ((e: Event) => void) | null = null;
  onclose: ((e: Event) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  sent: any[] = [];

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  send(data: any) { this.sent.push(data); }
  close() { this.readyState = 3; this.onclose?.(new Event('close')); }

  // Test helpers
  triggerOpen() { this.readyState = 1; this.onopen?.(new Event('open')); }
  triggerMessage(data: any) { this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) })); }
}

beforeEach(() => {
  MockWebSocket.instances = [];
  (globalThis as any).WebSocket = MockWebSocket;
});
afterEach(() => { vi.restoreAllMocks(); });

describe('PodLogPanel', () => {
  it('opens WebSocket to /api/v1/k8s/clusters/.../pods/.../logs/stream', () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0].url).toContain('/api/v1/k8s/clusters/c1/namespaces/default/pods/nginx-abc/logs/stream');
  });

  it('appends received log lines to display', async () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    const ws = MockWebSocket.instances[0];
    ws.triggerOpen();
    ws.triggerMessage({ line: 'nginx started', ts: '2026-06-13T10:00:00Z' });
    ws.triggerMessage({ line: 'listening on :80', ts: '2026-06-13T10:00:01Z' });
    await waitFor(() => {
      expect(screen.getByText(/nginx started/)).toBeInTheDocument();
      expect(screen.getByText(/listening on :80/)).toBeInTheDocument();
    });
  });

  it('closes WebSocket on unmount', () => {
    const { unmount } = render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" />);
    const ws = MockWebSocket.instances[0];
    const closeSpy = vi.spyOn(ws, 'close');
    unmount();
    expect(closeSpy).toHaveBeenCalled();
  });

  it('appends container query param when provided', () => {
    render(<PodLogPanel clusterID="c1" namespace="default" podName="nginx-abc" container="nginx" />);
    expect(MockWebSocket.instances[0].url).toContain('?container=nginx');
  });
});
