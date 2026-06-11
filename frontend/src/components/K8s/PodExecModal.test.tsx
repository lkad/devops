// PodExecModal test — covers the exec input piping and the
// reconnect-on-close retry path. We mock apiPost to return a
// fixed sessionId, and provide a controllable FakeWS.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act, fireEvent } from '@testing-library/react';

const termInstances: { write: ReturnType<typeof vi.fn>; writeln: ReturnType<typeof vi.fn> }[] = [];

vi.mock('@xterm/xterm', () => {
  class FakeTerminal {
    cols = 80;
    rows = 24;
    write = vi.fn();
    writeln = vi.fn();
    open() { /* noop */ }
    loadAddon() { /* noop */ }
    dispose() { /* noop */ }
    onData(_h: any) { /* noop */ }
    onResize(_h: any) { /* noop */ }
    constructor() {
      termInstances.push({ write: this.write, writeln: this.writeln });
    }
  }
  return { Terminal: FakeTerminal };
});

vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class { fit() {} },
}));

vi.mock('../../api/client', () => ({
  getToken: () => 'test-token',
  setToken: () => {},
  apiPost: vi.fn().mockResolvedValue({ sessionId: 'sess-xyz' }),
  apiGet: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  api: vi.fn(),
}));

import { PodExecModal } from './PodExecModal';

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
  openOk() { this.readyState = 1; this.onopen?.({}); }
  deliver(data: any) { this.onmessage?.({ data }); }
  fail() { this.onerror?.({}); this.onclose?.({}); }
}

beforeEach(() => {
  FakeWS.instances = [];
  termInstances.length = 0;
  (globalThis as any).WebSocket = FakeWS as any;
  (globalThis as any).EventSource = class { close() {} addEventListener() {} removeEventListener() {} } as any;
  // Provide a ResizeObserver stub for jsdom.
  (globalThis as any).ResizeObserver = class { observe() {} disconnect() {} unobserve() {} };
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('PodExecModal', () => {
  it('renders the title and close button when open', () => {
    render(
      <PodExecModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="default"
        pod="nginx-abc"
      />,
    );
    expect(screen.getByText(/Shell: nginx-abc/)).toBeInTheDocument();
    expect(screen.getByTestId('pod-exec-close')).toBeInTheDocument();
  });

  it('renders nothing when closed', () => {
    const { container } = render(
      <PodExecModal
        open={false}
        onClose={() => {}}
        clusterId="c-1"
        namespace="default"
        pod="nginx-abc"
      />,
    );
    expect(container.querySelector('[data-testid="pod-exec-modal"]')).toBeNull();
  });

  it('POSTs to /exec to get a session id and opens a WS to /exec/:sessionId/ws', async () => {
    render(
      <PodExecModal
        open
        onClose={() => {}}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    await waitFor(() => expect(FakeWS.instances.length).toBe(1));
    const ws = FakeWS.instances[0];
    expect(ws.url).toMatch(/\/api\/v1\/k8s\/clusters\/c-1\/namespaces\/ns1\/pods\/pod-1\/exec\/sess-xyz\/ws/);
  });

  it('writes WS text frames into the terminal', async () => {
    render(
      <PodExecModal
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
    act(() => ws.deliver('hello shell'));
    await waitFor(() => {
      const writes = termInstances.flatMap((t) => t.write.mock.calls.map((c) => c[0]));
      expect(writes).toContain('hello shell');
    });
  });

  it('attempts to reconnect with backoff after an unexpected close', async () => {
    render(
      <PodExecModal
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
    // Force a drop. The modal should schedule a reconnect.
    act(() => ws.close());
    // status should flip to 'reconnecting' synchronously.
    await waitFor(() => expect(screen.getByTestId('pod-exec-status').textContent).toBe('reconnecting'));
    // Wait past the 500ms base backoff for the retry to fire.
    await waitFor(() => expect(FakeWS.instances.length).toBe(2), { timeout: 3000 });
    expect(FakeWS.instances[1].url).toMatch(/sess-xyz\/ws/);
  });

  it('calls onClose when the close button is clicked', async () => {
    const onClose = vi.fn();
    render(
      <PodExecModal
        open
        onClose={onClose}
        clusterId="c-1"
        namespace="ns1"
        pod="pod-1"
      />,
    );
    fireEvent.click(screen.getByTestId('pod-exec-close'));
    expect(onClose).toHaveBeenCalled();
  });
});
