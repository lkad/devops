import { create } from 'zustand'
import { apiClient } from '@/api/client'

export interface PhysicalHost {
  id: string
  name: string
  status: 'online' | 'monitoring_issue' | 'offline'
  cpu: number
  memory: number
  disk: number
  services: number
  lastSeen: string
  ipAddress?: string
  dataCenter?: string
}

interface PhysicalHostState {
  hosts: PhysicalHost[]
  currentHost: PhysicalHost | null
  isLoading: boolean
  error: string | null
  fetchHosts: () => Promise<void>
  fetchHost: (id: string) => Promise<void>
  createHost: (data: Partial<PhysicalHost>) => Promise<PhysicalHost>
  updateHost: (id: string, data: Partial<PhysicalHost>) => Promise<PhysicalHost>
  deleteHost: (id: string) => Promise<void>
  setCurrentHost: (host: PhysicalHost | null) => void
}

export const usePhysicalHostStore = create<PhysicalHostState>((set) => ({
  hosts: [],
  currentHost: null,
  isLoading: false,
  error: null,

  fetchHosts: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ hosts: PhysicalHost[] }>('/api/v1/physical-hosts')
      set({ hosts: response.hosts || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchHost: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const host = await apiClient.get<PhysicalHost>(`/api/v1/physical-hosts/${id}`)
      set({ currentHost: host, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createHost: async (data: Partial<PhysicalHost>) => {
    set({ isLoading: true, error: null })
    try {
      const host = await apiClient.post<PhysicalHost>('/api/v1/physical-hosts', data)
      set((state) => ({
        hosts: [...state.hosts, host],
        isLoading: false,
      }))
      return host
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateHost: async (id: string, data: Partial<PhysicalHost>) => {
    set({ isLoading: true, error: null })
    try {
      const host = await apiClient.put<PhysicalHost>(`/api/v1/physical-hosts/${id}`, data)
      set((state) => ({
        hosts: state.hosts.map((h) => (h.id === id ? host : h)),
        currentHost: state.currentHost?.id === id ? host : state.currentHost,
        isLoading: false,
      }))
      return host
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deleteHost: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/physical-hosts/${id}`)
      set((state) => ({
        hosts: state.hosts.filter((h) => h.id !== id),
        currentHost: state.currentHost?.id === id ? null : state.currentHost,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  setCurrentHost: (host: PhysicalHost | null) => {
    set({ currentHost: host })
  },
}))