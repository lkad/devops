// MetricsPanel test — covers the loading / success / error
// paths and the data_status badge. Mocks apiGet so the test
// runs offline.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MetricsPanel } from './MetricsPanel';

// Mock the apiGet function from client.ts so the panel can
// run without a real network.
vi.mock('../../api/client', () => ({
  apiGet: vi.fn(),
  getToken: () => 'test-token',
  setToken: () => {},
}));

import { apiGet } from '../../api/client';

describe('MetricsPanel', () => {
  beforeEach(() => {
    (apiGet as any).mockReset();
  });

  it('shows the loading state on first render', () => {
    (apiGet as any).mockReturnValue(new Promise(() => {})); // never resolves
    render(<MetricsPanel hostId="h-1" />);
    expect(screen.getByText(/loading metrics/i)).toBeInTheDocument();
  });

  it('renders CPU/Memory/Disk when data_status is fresh', async () => {
    (apiGet as any).mockResolvedValue({
      cpu: { cores: 8, usage_percent: 25.0 },
      memory: { total_mib: 16384, used_mib: 4096, usage_percent: 25.0 },
      disk: { disks: [{ mount: '/', size_gb: 100, used_gb: 30, usage_percent: 30.0 }] },
      uptime: { seconds: 3600, formatted: 'up 1 hour' },
      data_status: 'fresh',
    });
    render(<MetricsPanel hostId="h-1" />);
    await waitFor(() => expect(screen.getByText(/fresh/i)).toBeInTheDocument());
    expect(screen.getByText(/8 cores/)).toBeInTheDocument();
    // 4096 MiB is rendered as "4.0 GiB" by fmtBytesMiB.
    expect(screen.getByText(/4\.0 GiB/)).toBeInTheDocument();
  });

  it('renders a stale badge when data_status is stale', async () => {
    (apiGet as any).mockResolvedValue({
      cpu: { cores: 0, usage_percent: 0 },
      memory: { total_mib: 0, used_mib: 0, usage_percent: 0 },
      disk: { disks: [] },
      uptime: { seconds: 0, formatted: '' },
      data_status: 'stale',
      warnings: ['df -BG: connection refused'],
    });
    render(<MetricsPanel hostId="h-1" />);
    await waitFor(() => expect(screen.getByText(/stale/i)).toBeInTheDocument());
    expect(screen.getByText(/1 warning/)).toBeInTheDocument();
  });

  it('shows an error message when the fetch fails', async () => {
    (apiGet as any).mockRejectedValue(new Error('boom'));
    render(<MetricsPanel hostId="h-1" />);
    await waitFor(() => expect(screen.getByText(/metrics: boom/i)).toBeInTheDocument());
  });

  it('renders unavailable state with a "no data" hint', async () => {
    (apiGet as any).mockResolvedValue({
      cpu: { cores: 0, usage_percent: 0 },
      memory: { total_mib: 0, used_mib: 0, usage_percent: 0 },
      disk: { disks: [] },
      uptime: { seconds: 0, formatted: '' },
      data_status: 'unavailable',
    });
    render(<MetricsPanel hostId="h-1" />);
    await waitFor(() => expect(screen.getByText(/unavailable/i)).toBeInTheDocument());
    expect(screen.getByText(/no data available/i)).toBeInTheDocument();
  });
});
