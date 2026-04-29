import { apiClient } from '../client'

export interface Pipeline {
  id: string
  name: string
  status: string
  lastRun: string | null
  stages: string[]
  createdAt: string
}

export interface CreatePipelineRequest {
  name: string
  stages?: string[]
}

export interface UpdatePipelineRequest {
  name?: string
  stages?: string[]
}

export interface PipelineListResponse {
  pipelines: Pipeline[]
  total: number
}

export interface PipelineRun {
  id: string
  pipelineId: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
  startedAt: string
  finishedAt?: string
}

export const pipelinesApi = {
  list: () =>
    apiClient.get<PipelineListResponse>('/api/v1/pipelines'),

  get: (id: string) =>
    apiClient.get<Pipeline>(`/api/v1/pipelines/${id}`),

  create: (data: CreatePipelineRequest) =>
    apiClient.post<Pipeline>('/api/v1/pipelines', data),

  update: (id: string, data: UpdatePipelineRequest) =>
    apiClient.put<Pipeline>(`/api/v1/pipelines/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<void>(`/api/v1/pipelines/${id}`),

  execute: (id: string) =>
    apiClient.post<PipelineRun>(`/api/v1/pipelines/${id}/execute`, {}),

  getRuns: (id: string) =>
    apiClient.get<{ runs: PipelineRun[] }>(`/api/v1/pipelines/${id}/runs`),

  cancelRun: (runId: string) =>
    apiClient.post<void>(`/api/v1/pipeline-runs/${runId}/cancel`, {}),

  getRun: (runId: string) =>
    apiClient.get<PipelineRun>(`/api/v1/pipeline-runs/${runId}`),
}