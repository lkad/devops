import { create } from 'zustand'
import { apiClient } from '@/api/client'

export interface Device {
  id: string
  name: string
  type: string
  status: string
  dataCenter: string
  ipAddress: string
  lastSeen: string
  createdAt: string
  environment?: string
  labels?: Record<string, string>
}

interface DeviceState {
  devices: Device[]
  currentDevice: Device | null
  isLoading: boolean
  error: string | null
  fetchDevices: () => Promise<void>
  fetchDevice: (id: string) => Promise<void>
  createDevice: (data: Partial<Device>) => Promise<Device>
  updateDevice: (id: string, data: Partial<Device>) => Promise<Device>
  deleteDevice: (id: string) => Promise<void>
  setCurrentDevice: (device: Device | null) => void
}

export const useDeviceStore = create<DeviceState>((set) => ({
  devices: [],
  currentDevice: null,
  isLoading: false,
  error: null,

  fetchDevices: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ devices: Device[] }>('/api/v1/devices')
      set({ devices: response.devices || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchDevice: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const device = await apiClient.get<Device>(`/api/v1/devices/${id}`)
      set({ currentDevice: device, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createDevice: async (data: Partial<Device>) => {
    set({ isLoading: true, error: null })
    try {
      const device = await apiClient.post<Device>('/api/v1/devices', data)
      set((state) => ({
        devices: [...state.devices, device],
        isLoading: false,
      }))
      return device
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateDevice: async (id: string, data: Partial<Device>) => {
    set({ isLoading: true, error: null })
    try {
      const device = await apiClient.put<Device>(`/api/v1/devices/${id}`, data)
      set((state) => ({
        devices: state.devices.map((d) => (d.id === id ? device : d)),
        currentDevice: state.currentDevice?.id === id ? device : state.currentDevice,
        isLoading: false,
      }))
      return device
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deleteDevice: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/devices/${id}`)
      set((state) => ({
        devices: state.devices.filter((d) => d.id !== id),
        currentDevice: state.currentDevice?.id === id ? null : state.currentDevice,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  setCurrentDevice: (device: Device | null) => {
    set({ currentDevice: device })
  },
}))