import { apiClient } from '../client'

export type AlertChannelType = 'slack' | 'webhook' | 'email' | 'log'

export interface AlertChannel {
  id: string
  name: string
  type: AlertChannelType
  config: Record<string, unknown>
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface AlertHistory {
  id: string
  channelId: string
  channelName: string
  channelType: AlertChannelType
  status: 'sent' | 'failed' | 'pending'
  message: string
  sentAt: string
  error?: string
}

export interface CreateChannelRequest {
  name: string
  type: AlertChannelType
  config: Record<string, unknown>
  enabled?: boolean
}

export interface ChannelListResponse {
  channels: AlertChannel[]
  total: number
}

export interface AlertHistoryResponse {
  history: AlertHistory[]
  total: number
}

export const alertsApi = {
  listChannels: () =>
    apiClient.get<ChannelListResponse>('/api/v1/alerts/channels'),

  createChannel: (data: CreateChannelRequest) =>
    apiClient.post<AlertChannel>('/api/v1/alerts/channels', data),

  deleteChannel: (id: string) =>
    apiClient.delete<void>(`/api/v1/alerts/channels/${id}`),

  trigger: (channelId: string, message: string) =>
    apiClient.post<void>('/api/v1/alerts/trigger', { channelId, message }),

  history: (params?: { channelId?: string; limit?: number }) => {
    const stringParams: Record<string, string> = {}
    if (params) {
      if (params.channelId) stringParams.channelId = params.channelId
      if (params.limit !== undefined) stringParams.limit = String(params.limit)
    }
    return apiClient.get<AlertHistoryResponse>('/api/v1/alerts/history', { params: stringParams })
  },
}