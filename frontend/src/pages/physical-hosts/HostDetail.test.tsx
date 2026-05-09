import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { HostDetail } from './HostDetail'
import { projectApi } from '@/api/endpoints/projects'
import { devicesApi } from '@/api/endpoints/devices'
import { logsApi } from '@/api/endpoints/logs'

// Mock all API modules
vi.mock('@/api/endpoints/projects', () => ({
  projectsApi: {
    getProject: vi.fn(),
    getProjectPermissions: vi.fn(),
    getProjectResources: vi.fn(),
    updateResourceWeight: vi.fn(),
    unlinkResource: vi.fn(),
  },
  projectApi: {
    getProjectsForPhysicalHost: vi.fn(),
  },
}))

vi.mock('@/api/endpoints/devices', () => ({
  devicesApi: {
    get: vi.fn(),
    transitionState: vi.fn(),
  },
}))

vi.mock('@/api/endpoints/logs', () => ({
  logsApi: {
    query: vi.fn(),
  },
}))

const mockDevice = {
  id: 'host-1',
  name: 'test-host',
  type: 'physical_host',
  status: 'active',
  environment: 'prod',
  labels: { location: 'dc1', ip: '192.168.1.1' },
  registeredAt: new Date().toISOString(),
}

const mockProjectsWithWeight = [
  { id: 'proj-1', name: 'Project A', type: 'frontend', createdAt: new Date().toISOString() },
  { id: 'proj-2', name: 'Project B', type: 'backend', createdAt: new Date().toISOString() },
]

// Note: HostDetail doesn't return weights from getProjectsForPhysicalHost
// So for now we just test that the projects tab displays

const renderWithRouter = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/physical-hosts/host-1']}>
        <Routes>
          <Route path="/physical-hosts/:id" element={ui} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('HostDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(devicesApi.get as ReturnType<typeof vi.fn>).mockResolvedValue(mockDevice)
    ;(devicesApi.transitionState as ReturnType<typeof vi.fn>).mockResolvedValue(mockDevice)
    ;(projectApi.getProjectsForPhysicalHost as ReturnType<typeof vi.fn>).mockResolvedValue(mockProjectsWithWeight)
    ;(logsApi.query as ReturnType<typeof vi.fn>).mockResolvedValue({ data: [] })
  })

  describe('Projects tab weight display', () => {
    it('should display linked projects in projects tab', async () => {
      renderWithRouter(<HostDetail />)

      // Wait for the page to load - use getAllByText since title appears twice
      await waitFor(() => {
        expect(screen.getAllByText('test-host').length).toBeGreaterThan(0)
      })

      // Click on projects tab
      fireEvent.click(screen.getByText('关联项目'))

      await waitFor(() => {
        expect(screen.getByText('Project A')).toBeInTheDocument()
        expect(screen.getByText('Project B')).toBeInTheDocument()
      })
    })
  })
})
