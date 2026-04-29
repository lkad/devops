import { create } from 'zustand'
import { apiClient } from '@/api/client'

export interface Resource {
  id: string
  type: 'device' | 'pipeline' | 'host'
  name: string
  weight: number
}

export interface Permission {
  id: string
  userId: string
  userName: string
  role: 'admin' | 'editor' | 'viewer'
  level: number
}

export interface Project {
  id: string
  name: string
  type?: string
  systemId: string
  systemName: string
  businessLineId: string
  businessLineName: string
  description?: string
  createdAt: string
}

export interface System {
  id: string
  name: string
  businessLineId: string
  businessLineName: string
  projects: Project[]
}

export interface BusinessLine {
  id: string
  name: string
  systems: System[]
}

interface ProjectState {
  businessLines: BusinessLine[]
  currentProject: Project | null
  resources: Resource[]
  permissions: Permission[]
  isLoading: boolean
  error: string | null
  fetchBusinessLines: () => Promise<void>
  fetchProject: (id: string) => Promise<void>
  createProject: (data: Partial<Project>) => Promise<Project>
  updateProject: (id: string, data: Partial<Project>) => Promise<Project>
  deleteProject: (id: string) => Promise<void>
  fetchResources: (projectId: string) => Promise<void>
  addResource: (projectId: string, resource: Partial<Resource>) => Promise<void>
  removeResource: (projectId: string, resourceId: string) => Promise<void>
  updateResourceWeight: (projectId: string, resourceId: string, weight: number) => Promise<void>
  fetchPermissions: (projectId: string) => Promise<void>
  addPermission: (projectId: string, permission: Partial<Permission>) => Promise<void>
  removePermission: (projectId: string, permissionId: string) => Promise<void>
  setCurrentProject: (project: Project | null) => void
}

export const useProjectStore = create<ProjectState>((set) => ({
  businessLines: [],
  currentProject: null,
  resources: [],
  permissions: [],
  isLoading: false,
  error: null,

  fetchBusinessLines: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ businessLines: BusinessLine[] }>('/api/v1/projects/tree')
      set({ businessLines: response.businessLines || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchProject: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const project = await apiClient.get<Project>(`/api/v1/projects/${id}`)
      set({ currentProject: project, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  createProject: async (data: Partial<Project>) => {
    set({ isLoading: true, error: null })
    try {
      const project = await apiClient.post<Project>('/api/v1/projects', data)
      set((state) => ({
        businessLines: state.businessLines.map((bl) => ({
          ...bl,
          systems: bl.systems.map((s) => ({
            ...s,
            projects: [...s.projects, project],
          })),
        })),
        isLoading: false,
      }))
      return project
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateProject: async (id: string, data: Partial<Project>) => {
    set({ isLoading: true, error: null })
    try {
      const project = await apiClient.put<Project>(`/api/v1/projects/${id}`, data)
      set((state) => ({
        businessLines: state.businessLines.map((bl) => ({
          ...bl,
          systems: bl.systems.map((s) => ({
            ...s,
            projects: s.projects.map((p) => (p.id === id ? project : p)),
          })),
        })),
        currentProject: state.currentProject?.id === id ? project : state.currentProject,
        isLoading: false,
      }))
      return project
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  deleteProject: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/projects/${id}`)
      set((state) => ({
        businessLines: state.businessLines.map((bl) => ({
          ...bl,
          systems: bl.systems.map((s) => ({
            ...s,
            projects: s.projects.filter((p) => p.id !== id),
          })),
        })),
        currentProject: state.currentProject?.id === id ? null : state.currentProject,
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  fetchResources: async (projectId: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ resources: Resource[] }>(`/api/v1/projects/${projectId}/resources`)
      set({ resources: response.resources || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  addResource: async (projectId: string, resource: Partial<Resource>) => {
    set({ isLoading: true, error: null })
    try {
      const result = await apiClient.post<Resource>(`/api/v1/projects/${projectId}/resources`, resource)
      set((state) => ({
        resources: [...state.resources, result],
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  removeResource: async (projectId: string, resourceId: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/projects/${projectId}/resources/${resourceId}`)
      set((state) => ({
        resources: state.resources.filter((r) => r.id !== resourceId),
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  updateResourceWeight: async (projectId: string, resourceId: string, weight: number) => {
    set({ isLoading: true, error: null })
    try {
      const result = await apiClient.put<Resource>(`/api/v1/projects/${projectId}/resources/${resourceId}`, { weight })
      set((state) => ({
        resources: state.resources.map((r) => (r.id === resourceId ? result : r)),
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  fetchPermissions: async (projectId: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ permissions: Permission[] }>(`/api/v1/projects/${projectId}/permissions`)
      set({ permissions: response.permissions || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  addPermission: async (projectId: string, permission: Partial<Permission>) => {
    set({ isLoading: true, error: null })
    try {
      const result = await apiClient.post<Permission>(`/api/v1/projects/${projectId}/permissions`, permission)
      set((state) => ({
        permissions: [...state.permissions, result],
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  removePermission: async (projectId: string, permissionId: string) => {
    set({ isLoading: true, error: null })
    try {
      await apiClient.delete(`/api/v1/projects/${projectId}/permissions/${permissionId}`)
      set((state) => ({
        permissions: state.permissions.filter((p) => p.id !== permissionId),
        isLoading: false,
      }))
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
      throw err
    }
  },

  setCurrentProject: (project: Project | null) => {
    set({ currentProject: project })
  },
}))