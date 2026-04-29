import { create } from 'zustand'
import { apiClient } from '@/api/client'

export type ClusterEnvironment = 'dev' | 'test' | 'uat' | 'prod'
export type ClusterHealth = 'healthy' | 'degraded' | 'unhealthy'

export interface K8sNode {
  name: string
  status: 'Ready' | 'NotReady'
  cpu: number
  memory: number
  age: string
  labels?: Record<string, string>
}

export interface K8sPod {
  name: string
  namespace: string
  status: 'Running' | 'Pending' | 'Succeeded' | 'Failed' | 'Unknown'
  cpu: number
  memory: number
  age: string
  node?: string
  containers?: string[]
}

export interface K8sNamespace {
  name: string
  status: string
  labels?: Record<string, string>
}

export interface K8sCluster {
  id: string
  name: string
  environment: ClusterEnvironment
  health: ClusterHealth
  version: string
  nodeCount: number
  podCount: number
  createdAt: string
}

interface K8sState {
  clusters: K8sCluster[]
  currentCluster: K8sCluster | null
  nodes: K8sNode[]
  pods: K8sPod[]
  namespaces: K8sNamespace[]
  isLoading: boolean
  error: string | null
  fetchClusters: () => Promise<void>
  fetchCluster: (id: string) => Promise<void>
  fetchNodes: (clusterId: string) => Promise<void>
  fetchPods: (clusterId: string, namespace?: string) => Promise<void>
  fetchNamespaces: (clusterId: string) => Promise<void>
  setCurrentCluster: (cluster: K8sCluster | null) => void
}

export const useK8sStore = create<K8sState>((set) => ({
  clusters: [],
  currentCluster: null,
  nodes: [],
  pods: [],
  namespaces: [],
  isLoading: false,
  error: null,

  fetchClusters: async () => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ clusters: K8sCluster[] }>('/api/v1/kubernetes/clusters')
      set({ clusters: response.clusters || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchCluster: async (id: string) => {
    set({ isLoading: true, error: null })
    try {
      const cluster = await apiClient.get<K8sCluster>(`/api/v1/kubernetes/clusters/${id}`)
      set({ currentCluster: cluster, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchNodes: async (clusterId: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ nodes: K8sNode[] }>(`/api/v1/kubernetes/clusters/${clusterId}/nodes`)
      set({ nodes: response.nodes || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchPods: async (clusterId: string, namespace?: string) => {
    set({ isLoading: true, error: null })
    try {
      const url = namespace
        ? `/api/v1/kubernetes/clusters/${clusterId}/pods?namespace=${namespace}`
        : `/api/v1/kubernetes/clusters/${clusterId}/pods`
      const response = await apiClient.get<{ pods: K8sPod[] }>(url)
      set({ pods: response.pods || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  fetchNamespaces: async (clusterId: string) => {
    set({ isLoading: true, error: null })
    try {
      const response = await apiClient.get<{ namespaces: K8sNamespace[] }>(`/api/v1/kubernetes/clusters/${clusterId}/namespaces`)
      set({ namespaces: response.namespaces || [], isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  setCurrentCluster: (cluster: K8sCluster | null) => {
    set({ currentCluster: cluster })
  },
}))