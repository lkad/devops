import { create } from 'zustand'
import { apiClient } from '@/api/client'

export interface LogEntry {
  id: string
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  source: string
  message: string
  metadata?: Record<string, unknown>
}

export interface LogQuery {
  startTime?: string
  endTime?: string
  level?: string
  source?: string
  query?: string
}

export interface AlertRule {
  id: string
  name: string
  condition: string
  severity: 'critical' | 'warning' | 'info'
  enabled: boolean
  channels: string[]
  createdAt: string
}

interface LogState {
  logs: LogEntry[]
  alertRules: AlertRule[]
  currentRule: AlertRule | null
  isLoading: boolean
  isStreaming: boolean
  error: string | null
  query: LogQuery
  fetchLogs: (query?: LogQuery) => Promise<void>
  fetchAlertRules: () => Promise<void>
  createAlertRule: (data: Partial<AlertRule>) => Promise<AlertRule>
  updateAlertRule: (id: string, data: Partial<AlertRule>) => Promise<AlertRule>
  deleteAlertRule: (id: string) => Promise<void>
  setQuery: (query: LogQuery) => void
  addLog: (log: LogEntry) => void
  clearLogs: () => void
}

export const useLogStore = create<LogState>((set) => ({
  logs: [],
  alertRules: [],
  currentRule: null,
  isLoading: false,
  isStreaming: false,
  error: null,
  query: {},

  fetchLogs: async (query?: LogQuery) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ logs: LogEntry[] }>('/api/v1/logs', {
        params: query as Record<string, string>,
      })
      set({ logs: response.logs || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchAlertRules: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ rules: AlertRule[] }>('/api/v1/alert-rules')
      set({ alertRules: response.rules || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createAlertRule: async (data: Partial<AlertRule>) => {
    set({ isLoading: true, error: null })
    try {
      const rule = await apiClient.post<AlertRule>('/api/v1/alert-rules', data)
      set((state) => ({
        alertRules: [...state.alertRules, rule],
        isLoading: false,
      }))
      return rule
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateAlertRule: async (id: string, data: Partial<AlertRule>) => {
    set({ isLoading: true, error: null })
    try {
      const rule = await apiClient.put<AlertRule>(`/api/v1/alert-rules/${id}`, data)
      set((state) => ({
        alertRules: state.alertRules.map((r) => (r.id === id ? rule : r)),
        currentRule: state.currentRule?.id === id ? rule : state.currentRule,
        isLoading: false,
      }))
      return rule
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deleteAlertRule: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/alert-rules/${id}`)
      set((state) => ({
        alertRules: state.alertRules.filter((r) => r.id !== id),
        currentRule: state.currentRule?.id === id ? null : state.currentRule,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  setQuery: (query: LogQuery) => {
    set({ query })
  },

  addLog: (log: LogEntry) => {
    set((state) => ({
      logs: [log, ...state.logs].slice(0, 1000), // Keep last 1000 logs
    }))
  },

  clearLogs: () => {
    set({ logs: [] })
  },
}))