import { apiClient } from '../client'

export interface BusinessLine {
  id: string
  name: string
  description?: string
  systems?: System[]
  createdAt: string
}

export interface System {
  id: string
  businessLineId: string
  name: string
  description?: string
  projects?: Project[]
  createdAt: string
}

export interface Project {
  id: string
  systemId: string
  name: string
  type: 'frontend' | 'backend'
  description?: string
  createdAt: string
}

export interface CreateBusinessLineRequest {
  name: string
  description?: string
}

export interface UpdateBusinessLineRequest {
  name?: string
  description?: string
}

export interface CreateSystemRequest {
  name: string
  description?: string
}

export interface UpdateSystemRequest {
  name?: string
  description?: string
}

export interface CreateProjectRequest {
  name: string
  type: 'frontend' | 'backend'
  description?: string
}

export interface UpdateProjectRequest {
  name?: string
  type?: 'frontend' | 'backend'
  description?: string
}

export interface ProjectTreeResponse {
  businessLines: Array<BusinessLine & { systems: Array<System & { projects: Project[] }> }>
}

export const projectsApi = {
  // Business Lines
  listBusinessLines: () =>
    apiClient.get<BusinessLine[]>('/api/org/business-lines'),

  getBusinessLine: (id: string) =>
    apiClient.get<BusinessLine>(`/api/org/business-lines/${id}`),

  createBusinessLine: (data: CreateBusinessLineRequest) =>
    apiClient.post<BusinessLine>('/api/org/business-lines', data),

  updateBusinessLine: (id: string, data: UpdateBusinessLineRequest) =>
    apiClient.put<BusinessLine>(`/api/org/business-lines/${id}`, data),

  deleteBusinessLine: (id: string) =>
    apiClient.delete<void>(`/api/org/business-lines/${id}`),

  // Systems
  listSystems: (blId: string) =>
    apiClient.get<System[]>(`/api/org/business-lines/${blId}/systems`),

  createSystem: (blId: string, data: CreateSystemRequest) =>
    apiClient.post<System>(`/api/org/business-lines/${blId}/systems`, data),

  getSystem: (id: string) =>
    apiClient.get<System>(`/api/org/systems/${id}`),

  updateSystem: (id: string, data: UpdateSystemRequest) =>
    apiClient.put<System>(`/api/org/systems/${id}`, data),

  deleteSystem: (id: string) =>
    apiClient.delete<void>(`/api/org/systems/${id}`),

  // Projects
  listProjects: (sysId: string) =>
    apiClient.get<Project[]>(`/api/org/systems/${sysId}/projects`),

  createProject: (sysId: string, data: CreateProjectRequest) =>
    apiClient.post<Project>(`/api/org/systems/${sysId}/projects`, data),

  getProject: (id: string) =>
    apiClient.get<Project>(`/api/org/projects/${id}`),

  updateProject: (id: string, data: UpdateProjectRequest) =>
    apiClient.put<Project>(`/api/org/projects/${id}`, data),

  deleteProject: (id: string) =>
    apiClient.delete<void>(`/api/org/projects/${id}`),

  // Tree view
  getProjectTree: () =>
    apiClient.get<ProjectTreeResponse>('/api/v1/projects/tree'),
}