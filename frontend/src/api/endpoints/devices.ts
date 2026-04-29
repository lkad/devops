import { apiClient } from '../client'

export interface Device {
  id: string
  name: string
  type: string
  status: string
  environment: string
  labels: Record<string, string>
  registeredAt?: string
  lastSeen?: string
  dataCenter?: string
  ipAddress?: string
}

export interface CreateDeviceRequest {
  name: string
  type: string
  environment?: string
  labels?: Record<string, string>
  status?: string
  dataCenter?: string
  ipAddress?: string
}

export interface UpdateDeviceRequest {
  name?: string
  type?: string
  environment?: string
  labels?: Record<string, string>
  status?: string
  dataCenter?: string
  ipAddress?: string
}

export interface DeviceListResponse {
  data: Device[]
  total?: number
  pagination?: {
    total: number
    limit: number
    offset: number
    has_more: boolean
  }
}

export const devicesApi = {
  list: (params?: { environment?: string; status?: string }) =>
    apiClient.get<DeviceListResponse>('/api/devices', { params }),

  get: (id: string) =>
    apiClient.get<Device>(`/api/devices/${id}`),

  create: (data: CreateDeviceRequest) =>
    apiClient.post<Device>('/api/devices', data),

  update: (id: string, data: UpdateDeviceRequest) =>
    apiClient.put<Device>(`/api/devices/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<void>(`/api/devices/${id}`),

  search: (query: string) =>
    apiClient.get<DeviceListResponse>('/api/devices/search', { params: { q: query } }),
}