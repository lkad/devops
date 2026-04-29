import { apiClient } from '../client'

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

export interface PhysicalHostListResponse {
  hosts: PhysicalHost[]
  total: number
}

export const physicalHostsApi = {
  list: () =>
    apiClient.get<PhysicalHostListResponse>('/api/v1/physical-hosts'),

  get: (id: string) =>
    apiClient.get<PhysicalHost>(`/api/v1/physical-hosts/${id}`),

  create: (data: Partial<PhysicalHost>) =>
    apiClient.post<PhysicalHost>('/api/v1/physical-hosts', data),

  update: (id: string, data: Partial<PhysicalHost>) =>
    apiClient.put<PhysicalHost>(`/api/v1/physical-hosts/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<void>(`/api/v1/physical-hosts/${id}`),
}