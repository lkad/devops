// PodLogsModal test — covers open/close flow and log line append.
// We mock global fetch and WebSocket so the test runs offline.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { PodLogsModal } from './PodLogsModal';

// Mock the api client module so token helpers are predictable.
vi.mock('../../api/client', () => ({
  getToken: () => 'test-token',
  setToken: () => {},
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  api: vi.fn(),
}));

// A minimal controllable WebSocket double. We don't subclass
// the global — we install a constructor and capture instances.
class FakeWS {
  static instances: FakeWS[] = [];
  static OPEN = 1;
  url: string;
  readyState = 0;
  onopen: ((ev?: any) => void) | null = null;
  onmessage: ((ev: any) => void) | null = null;
  onerror: ((ev?: any) => void) | null = null;
  onclose: ((ev?: any) => void) | null = null;
  sent: string[] = [];
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
  }
  send(data: string) { this.sent.push(data); }
  close() {
    this.readyState = 3;
    this.onclose?.({});
  }
  // helpers used by tests
  openOk() { this.readyState = 1; this.onopen?.({}); }
  deliver(data: any) { this.onmessage?.({ data }); }
  fail() { this.onerror?.({}); this.onclose?.({}); }
}

beforeEach(() => {
  FakeWS.instances = [];
  (globalThis as any).WebSocket = FakeWS as any;
  (globalThis as any).EventSource = class { close() {} onopen: any; onmessage: any; onerror: any; addEventListener() {} removeEventListener() {} } as any;
  // fetch returns a resolved response with 0 lines by default.
  (globalThis as any).fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({ data: { lines: [] } }),
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('PodLogsModal', () => {
  it('renders the title and a close button when open', async () => {
    render(
      <PodLogsModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="default"
        pod="nginx-abc"
      />,
    );
    expect(screen.getByText(/Logs: nginx-abc/)).toBeInTheDocument();
    expect(screen.getByTestId('pod-logs-close')).toBeInTheDocument();
  });

  it('renders nothing when closed', () => {
    render(
      <PodLogsModal
        open={false}
        onClose={() => {}}
        clusterId="c-1"
        namespace="default"
        pod="nginx-abc"
      />,
    );
    expect(screen.queryByTestId('pod-logs-modal')).toBeNull();
  });

  it('opens a WebSocket to the per-pod stream on mount', async () => {
    render(
      <PodLogsModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    await waitFor(() => {
      expect(FakeWS.instances.length).toBe(1);
    });
    expect(FakeWS.instances[0].url).toMatch(/\/api\/v1\/k8s\/clusters\/c-1\/pods\/ns1\/pod-1\/logs\/stream/);
  });

  it('appends WS frame lines into the buffer', async () => {
    render(
      <PodLogsModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    await waitFor(() => expect(FakeWS.instances.length).toBe(1));
    const ws = FakeWS.instances[0];
    act(() => ws.openOk());
    act(() => ws.deliver(JSON.stringify({ lines: ['hello', 'world'] })));
    await waitFor(() => {
      expect(screen.getAllByTestId('pod-logs-line').map((el) => el.textContent)).toEqual(['hello', 'world']);
    });
  });

  it('toggles the pause button and reflects the state in the footer', async () => {
    render(
      <PodLogsModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    const btn = screen.getByTestId('pod-logs-pause');
    expect(btn.textContent).toBe('Pause');
    fireEvent.click(btn);
    expect(btn.textContent).toBe('Resume');
    expect(screen.getByText(/scroll paused/i)).toBeInTheDocument();
  });

  it('calls onClose when the close button is clicked', async () => {
    const onClose = vi.fn();
    render(
      <PodLogsModal
        open
        onClose={onClose}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    fireEvent.click(screen.getByTestId('pod-logs-close'));
    expect(onClose).toHaveBeenCalled();
  });
});
