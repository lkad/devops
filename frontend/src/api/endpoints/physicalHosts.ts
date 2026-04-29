import { apiClient } from '../client'

export interface PhysicalHost {
  id: string
  hostname: string
  ip: string
  port: number
  state: 'online' | 'monitoring_issue' | 'offline'
  monitoringStatus: 'up' | 'down'
  lastHeartbeat?: string
  metrics?: {
    cpu: { usage: number; cores: number }
    memory: { total: number; used: number; usagePercent: number }
    disk: { disks: Array<{ device: string; total: number; used: number }> }
    uptime: { value: number; formatted: string }
  }
  registeredAt: string
}

export interface PhysicalHostListResponse {
  hosts: PhysicalHost[]
  total: number
}

export interface CreatePhysicalHostRequest {
  hostname: string
  ip: string
  port: number
}

export interface HostMetrics {
  cpu: { usage: number; cores: number }
  memory: { total: number; used: number; usagePercent: number }
  disk: { disks: Array<{ device: string; total: number; used: number }> }
  uptime: { value: number; formatted: string }
}

export interface HostConfig {
  content?: string
}

export const physicalHostsApi = {
  listHosts: () =>
    apiClient.get<PhysicalHostListResponse>('/api/physical-hosts'),

  getHost: (id: string) =>
    apiClient.get<PhysicalHost>(`/api/physical-hosts/${id}`),

  createHost: (data: CreatePhysicalHostRequest) =>
    apiClient.post<PhysicalHost>('/api/physical-hosts', data),

  deleteHost: (id: string) =>
    apiClient.delete<void>(`/api/physical-hosts/${id}`),

  getMetrics: (id: string) =>
    apiClient.get<HostMetrics>(`/api/physical-hosts/${id}/metrics`),

  getConfig: (id: string) =>
    apiClient.get<HostConfig>(`/api/physical-hosts/${id}/config`),

  pushConfig: (id: string, content: string) =>
    apiClient.post<void>(`/api/physical-hosts/${id}/config`, { content }),
}