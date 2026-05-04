import { apiClient } from '../client'

export interface LogEntry {
  id: string
  timestamp: string
  level: 'error' | 'warn' | 'info' | 'debug'
  source: string
  message: string
  metadata?: Record<string, unknown>
}

export interface LogQuery {
  query?: string
  level?: string
  source?: string
  device?: string
  startTime?: string
  endTime?: string
  limit?: number
  offset?: number
}

export interface LogStats {
  total: number
  byLevel: {
    error: number
    warn: number
    info: number
    debug: number
  }
  bySource: Record<string, number>
}

export interface LogQueryResponse {
  data: LogEntry[]
  pagination: {
    total: number
    limit: number
    offset: number
    has_more: boolean
  }
}

export interface LogStatsResponse {
  stats: LogStats
}

export interface GenerateLogsRequest {
  count: number
  level?: string
  source?: string
}

export interface GenerateLogsResponse {
  generated: number
}

export const logsApi = {
  query: (params?: LogQuery) => {
    const stringParams: Record<string, string> = {}
    if (params) {
      if (params.query) stringParams.query = params.query
      if (params.level) stringParams.level = params.level
      if (params.source) stringParams.source = params.source
      if (params.device) stringParams.device = params.device
      if (params.startTime) stringParams.startTime = params.startTime
      if (params.endTime) stringParams.endTime = params.endTime
      if (params.limit !== undefined) stringParams.limit = String(params.limit)
      if (params.offset !== undefined) stringParams.offset = String(params.offset)
    }
    return apiClient.get<LogQueryResponse>('/api/logs', { params: stringParams })
  },

  stats: () =>
    apiClient.get<LogStatsResponse>('/api/logs/stats'),

  generate: (data: GenerateLogsRequest) =>
    apiClient.post<GenerateLogsResponse>('/api/logs/generate', data),
}