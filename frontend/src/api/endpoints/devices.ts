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
  devices: Device[]
  total: number
}

export const devicesApi = {
  list: (params?: { environment?: string; status?: string }) =>
    apiClient.get<DeviceListResponse>('/api/v1/devices', { params }),

  get: (id: string) =>
    apiClient.get<Device>(`/api/v1/devices/${id}`),

  create: (data: CreateDeviceRequest) =>
    apiClient.post<Device>('/api/v1/devices', data),

  update: (id: string, data: UpdateDeviceRequest) =>
    apiClient.put<Device>(`/api/v1/devices/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<void>(`/api/v1/devices/${id}`),
}