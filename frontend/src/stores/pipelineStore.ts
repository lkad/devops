import { create } from 'zustand'
import { apiClient } from '@/api/client'

export interface Pipeline {
  id: string
  name: string
  status: string
  lastRun: string | null
  stages: string[]
  createdAt: string
}

export interface PipelineRun {
  id: string
  pipelineId: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  startedAt: string
  finishedAt?: string
  logs?: string
}

interface PipelineState {
  pipelines: Pipeline[]
  currentPipeline: Pipeline | null
  runs: PipelineRun[]
  currentRun: PipelineRun | null
  isLoading: boolean
  error: string | null
  fetchPipelines: () => Promise<void>
  fetchPipeline: (id: string) => Promise<void>
  createPipeline: (data: Partial<Pipeline>) => Promise<Pipeline>
  updatePipeline: (id: string, data: Partial<Pipeline>) => Promise<Pipeline>
  deletePipeline: (id: string) => Promise<void>
  executePipeline: (id: string) => Promise<PipelineRun>
  cancelRun: (runId: string) => Promise<void>
  fetchRuns: (pipelineId: string) => Promise<void>
  fetchRun: (runId: string) => Promise<void>
  setCurrentPipeline: (pipeline: Pipeline | null) => void
  setCurrentRun: (run: PipelineRun | null) => void
}

export const usePipelineStore = create<PipelineState>((set) => ({
  pipelines: [],
  currentPipeline: null,
  runs: [],
  currentRun: null,
  isLoading: false,
  error: null,

  fetchPipelines: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ pipelines: Pipeline[] }>('/api/v1/pipelines')
      set({ pipelines: response.pipelines || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchPipeline: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const pipeline = await apiClient.get<Pipeline>(`/api/v1/pipelines/${id}`)
      set({ currentPipeline: pipeline, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createPipeline: async (data: Partial<Pipeline>) => {
    set({ isLoading: true, error: null })
    try {
      const pipeline = await apiClient.post<Pipeline>('/api/v1/pipelines', data)
      set((state) => ({
        pipelines: [...state.pipelines, pipeline],
        isLoading: false,
      }))
      return pipeline
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updatePipeline: async (id: string, data: Partial<Pipeline>) => {
    set({ isLoading: true, error: null })
    try {
      const pipeline = await apiClient.put<Pipeline>(`/api/v1/pipelines/${id}`, data)
      set((state) => ({
        pipelines: state.pipelines.map((p) => (p.id === id ? pipeline : p)),
        currentPipeline: state.currentPipeline?.id === id ? pipeline : state.currentPipeline,
        isLoading: false,
      }))
      return pipeline
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deletePipeline: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/pipelines/${id}`)
      set((state) => ({
        pipelines: state.pipelines.filter((p) => p.id !== id),
        currentPipeline: state.currentPipeline?.id === id ? null : state.currentPipeline,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  executePipeline: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const run = await apiClient.post<PipelineRun>(`/api/v1/pipelines/${id}/execute`, {})
      set((state) => ({
        runs: [...state.runs, run],
        isLoading: false,
      }))
      return run
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  cancelRun: async (runId: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.post(`/api/v1/pipeline-runs/${runId}/cancel`, {})
      set((state) => ({
        runs: state.runs.map((r) =>
          r.id === runId ? { ...r, status: 'cancelled' as const } : r
        ),
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  fetchRuns: async (pipelineId: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ runs: PipelineRun[] }>(`/api/v1/pipelines/${pipelineId}/runs`)
      set({ runs: response.runs || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchRun: async (runId: string) => {
    set({ isLoading: true, error: null })
    try {
      const run = await apiClient.get<PipelineRun>(`/api/v1/pipeline-runs/${runId}`)
      set({ currentRun: run, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  setCurrentPipeline: (pipeline: Pipeline | null) => {
    set({ currentPipeline: pipeline })
  },

  setCurrentRun: (run: PipelineRun | null) => {
    set({ currentRun: run })
  },
}))