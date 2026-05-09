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
  config?: Record<string, unknown>
  metadata?: Record<string, unknown>
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
}

export interface TransitionStateRequest {
  state: string
  triggered_by: string
  reason?: string
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

// K8s types
export interface K8sNode {
  name: string
  ready: boolean
  role: string
  cpu: string
  memory: string
  age: string
  taints?: string[]
  labels?: Record<string, string>
  condition: string
}

export interface K8sPod {
  name: string
  namespace: string
  ready: string
  status: string
  restarts: number
  cpu: string
  memory: string
  age: string
  node_name: string
  ip: string
}

export const devicesApi = {
  list: (params?: { environment?: string; status?: string; type?: string }) =>
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

  transitionState: (id: string, data: TransitionStateRequest) =>
    apiClient.put<Device>(`/api/devices/${id}/state`, data),

  // K8s cluster specific operations
  getDeviceNodes: (id: string) =>
    apiClient.get<K8sNode[]>(`/api/devices/${id}/nodes`),

  getDevicePods: (id: string, namespace?: string) =>
    apiClient.get<K8sPod[]>(`/api/devices/${id}/pods`, {
      params: namespace ? { namespace } : undefined,
    }),

  getDeviceNamespaces: (id: string) =>
    apiClient.get<string[]>(`/api/devices/${id}/namespaces`),
}