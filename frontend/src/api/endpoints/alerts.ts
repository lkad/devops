import { apiClient } from '../client'

export type AlertChannelType = 'slack' | 'webhook' | 'email' | 'log'

export interface AlertChannel {
  name: string
  type: AlertChannelType
  config: Record<string, string>
  enabled: boolean
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

export interface ChannelListResponse {
  channels: AlertChannel[]
}

export interface AlertHistoryResponse {
  history: AlertHistory[]
  pagination?: {
    total: number
    limit: number
    offset: number
    has_more: boolean
  }
}

export const alertsApi = {
  listChannels: () =>
    apiClient.get<ChannelListResponse>('/api/alerts/channels'),

  createChannel: (data: Omit<AlertChannel, 'id' | 'createdAt'>) =>
    apiClient.post<AlertChannel>('/api/alerts/channels', data),

  updateChannel: (name: string, data: Partial<AlertChannel>) =>
    apiClient.put<AlertChannel>(`/api/alerts/channels/${name}`, data),

  deleteChannel: (name: string) =>
    apiClient.delete(`/api/alerts/channels/${name}`),

  testChannel: (name: string) =>
    apiClient.post(`/api/alerts/channels/${name}/test`, {}),

  getHistory: (params?: { limit?: number; offset?: number }) => {
    const searchParams: Record<string, string> = {}
    if (params?.limit) searchParams.limit = params.limit.toString()
    if (params?.offset) searchParams.offset = params.offset.toString()
    return apiClient.get<AlertHistoryResponse>('/api/alerts/history', {
      params: Object.keys(searchParams).length > 0 ? searchParams : undefined,
    })
  },
}