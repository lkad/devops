import { create } from 'zustand'
import { apiClient } from '@/api/client'

export type ChannelType = 'slack' | 'webhook' | 'email' | 'log'

export interface AlertChannel {
  id: string
  name: string
  type: ChannelType
  config: Record<string, unknown>
  enabled: boolean
  createdAt: string
}

export interface AlertHistoryEntry {
  id: string
  channelId: string
  channelName: string
  alertName: string
  status: 'triggered' | 'resolved'
  triggeredAt: string
  resolvedAt?: string
  message: string
}

interface AlertState {
  channels: AlertChannel[]
  currentChannel: AlertChannel | null
  history: AlertHistoryEntry[]
  isLoading: boolean
  error: string | null
  fetchChannels: () => Promise<void>
  fetchChannel: (id: string) => Promise<void>
  createChannel: (data: Partial<AlertChannel>) => Promise<AlertChannel>
  updateChannel: (id: string, data: Partial<AlertChannel>) => Promise<AlertChannel>
  deleteChannel: (id: string) => Promise<void>
  testChannel: (id: string) => Promise<void>
  fetchHistory: (params?: { startTime?: string; endTime?: string; channelId?: string }) => Promise<void>
  setCurrentChannel: (channel: AlertChannel | null) => void
}

export const useAlertStore = create<AlertState>((set) => ({
  channels: [],
  currentChannel: null,
  history: [],
  isLoading: false,
  error: null,

  fetchChannels: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ channels: AlertChannel[] }>('/api/v1/alert-channels')
      set({ channels: response.channels || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchChannel: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const channel = await apiClient.get<AlertChannel>(`/api/v1/alert-channels/${id}`)
      set({ currentChannel: channel, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createChannel: async (data: Partial<AlertChannel>) => {
    set({ isLoading: true, error: null })
    try {
      const channel = await apiClient.post<AlertChannel>('/api/v1/alert-channels', data)
      set((state) => ({
        channels: [...state.channels, channel],
        isLoading: false,
      }))
      return channel
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateChannel: async (id: string, data: Partial<AlertChannel>) => {
    set({ isLoading: true, error: null })
    try {
      const channel = await apiClient.put<AlertChannel>(`/api/v1/alert-channels/${id}`, data)
      set((state) => ({
        channels: state.channels.map((c) => (c.id === id ? channel : c)),
        currentChannel: state.currentChannel?.id === id ? channel : state.currentChannel,
        isLoading: false,
      }))
      return channel
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deleteChannel: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/alert-channels/${id}`)
      set((state) => ({
        channels: state.channels.filter((c) => c.id !== id),
        currentChannel: state.currentChannel?.id === id ? null : state.currentChannel,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  testChannel: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.post(`/api/v1/alert-channels/${id}/test`, {})
      set({ isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  fetchHistory: async (params) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ history: AlertHistoryEntry[] }>('/api/v1/alert-history', {
        params: params as Record<string, string>,
      })
      set({ history: response.history || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  setCurrentChannel: (channel: AlertChannel | null) => {
    set({ currentChannel: channel })
  },
}))