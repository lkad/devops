import { apiClient } from '../client'

export interface Pipeline {
  id: string
  name: string
  stages: string[]
  deployConfig: {
    strategy: 'blue_green' | 'canary' | 'rolling'
    canary?: { steps: number[] }
  }
  createdAt: string
}

export interface CreatePipelineRequest {
  name: string
  stages: string[]
  deployConfig: {
    strategy: 'blue_green' | 'canary' | 'rolling'
    canary?: { steps: number[] }
  }
}

export interface UpdatePipelineRequest {
  name?: string
  stages?: string[]
  deployConfig?: {
    strategy: 'blue_green' | 'canary' | 'rolling'
    canary?: { steps: number[] }
  }
}

export interface PipelineListResponse {
  pipelines: Pipeline[]
  total: number
}

export interface PipelineRun {
  id: string
  pipelineId: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  stage: string
  startedAt: string
  finishedAt?: string
  logs: string[]
}

export interface PipelineRunsResponse {
  runs: PipelineRun[]
}

export const pipelinesApi = {
  list: () =>
    apiClient.get<PipelineListResponse>('/api/pipelines'),

  get: (id: string) =>
    apiClient.get<Pipeline>(`/api/pipelines/${id}`),

  create: (data: CreatePipelineRequest) =>
    apiClient.post<Pipeline>('/api/pipelines', data),

  update: (id: string, data: UpdatePipelineRequest) =>
    apiClient.put<Pipeline>(`/api/pipelines/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<void>(`/api/pipelines/${id}`),

  execute: (id: string) =>
    apiClient.post<PipelineRun>(`/api/pipelines/${id}/execute`, {}),

  getRuns: (id: string) =>
    apiClient.get<PipelineRunsResponse>(`/api/pipelines/${id}/runs`),

  getRun: async (pipelineId: string, runId: string): Promise<PipelineRun> => {
    const response = await apiClient.get<PipelineRunsResponse>(`/api/pipelines/${pipelineId}/runs`)
    const run = response.runs.find(r => r.id === runId)
    if (!run) throw new Error('Run not found')
    return run
  },

  cancelRun: async (pipelineId: string, runId: string): Promise<void> => {
    // Backend may not have cancel endpoint yet - using execute response format
    return apiClient.post<void>(`/api/pipelines/${pipelineId}/cancel`, { runId })
  },
}