import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { ProjectDetail } from './ProjectDetail'
import { projectsApi } from '@/api/endpoints/projects'

// Mock the API module
vi.mock('@/api/endpoints/projects', () => ({
  projectsApi: {
    getProject: vi.fn(),
    getProjectPermissions: vi.fn(),
    getProjectResources: vi.fn(),
    updateResourceWeight: vi.fn(),
    deleteProject: vi.fn(),
  },
}))

const mockProject = {
  id: 'proj-1',
  systemId: 'sys-1',
  name: 'Test Project',
  type: 'frontend' as const,
  description: 'A test project',
  createdAt: new Date().toISOString(),
}

const mockResources = {
  data: [
    { id: 'res-1', resource_type: 'device', resource_id: 'dev-1', weight: 0.5 },
    { id: 'res-2', resource_type: 'pipeline', resource_id: 'pipe-1', weight: 0.3 },
  ],
}

const renderWithRouter = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/projects/proj-1']}>
        <Routes>
          <Route path="/projects/:id" element={ui} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ProjectDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Default mocks - will be overridden per test as needed
    ;(projectsApi.getProject as ReturnType<typeof vi.fn>).mockResolvedValue(mockProject)
    ;(projectsApi.getProjectResources as ReturnType<typeof vi.fn>).mockResolvedValue(mockResources)
    ;(projectsApi.getProjectPermissions as ReturnType<typeof vi.fn>).mockResolvedValue([])
    ;(projectsApi.updateResourceWeight as ReturnType<typeof vi.fn>).mockResolvedValue({})
  })

  describe('Resource weight inline editing', () => {
    it('should display weight as editable input field', async () => {
      renderWithRouter(<ProjectDetail />)

      // Should find weight input fields (one per resource row)
      const weightInputs = await screen.findAllByRole('spinbutton')
      expect(weightInputs.length).toBe(2) // Two resources

      // First resource has weight 0.5 (50%)
      expect(weightInputs[0]).toHaveValue(50)
      // Second resource has weight 0.3 (30%)
      expect(weightInputs[1]).toHaveValue(30)
    })

    it('should persist weight change when input changes', async () => {
      ;(projectsApi.updateResourceWeight as ReturnType<typeof vi.fn>).mockResolvedValue({})

      renderWithRouter(<ProjectDetail />)

      const weightInputs = await screen.findAllByRole('spinbutton')

      // Change weight from 50 to 75
      fireEvent.change(weightInputs[0], { target: { value: '75' } })

      // Wait for mutation to be called (mutation is async)
      await new Promise(resolve => setTimeout(resolve, 100))

      expect(projectsApi.updateResourceWeight).toHaveBeenCalledWith(
        'proj-1',
        'device',
        'dev-1',
        0.75
      )
    })

    it('should show total weight and warning when weights exceed 100%', async () => {
      const overWeightResources = {
        data: [
          { id: 'res-1', resource_type: 'device', resource_id: 'dev-1', weight: 0.7 },
          { id: 'res-2', resource_type: 'pipeline', resource_id: 'pipe-1', weight: 0.5 },
        ],
      }
      ;(projectsApi.getProjectResources as ReturnType<typeof vi.fn>).mockResolvedValue(overWeightResources)

      renderWithRouter(<ProjectDetail />)

      // Should show "Warning: Over 100%" because 0.7 + 0.5 = 1.2 (120%)
      await waitFor(() => {
        expect(screen.getByText(/warning.*over.*100%/i)).toBeInTheDocument()
      })
    })
  })
})
