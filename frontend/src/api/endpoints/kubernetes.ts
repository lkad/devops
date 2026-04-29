import { apiClient } from '../client'

export interface K8sCluster {
  id: string
  name: string
  type: string
  status: string
  version: string
  environment: string
  nodes: number
  pods: number
  namespaces: number
  createdAt?: string
  lastSeen?: string
}

export interface K8sNode {
  id: string
  name: string
  status: string
  role: string
  age: string
  version: string
}

export interface K8sPod {
  id: string
  name: string
  namespace: string
  status: string
  node: string
  age: string
}

export interface K8sNamespace {
  id: string
  name: string
  status: string
  labels: Record<string, string>
}

export interface ClusterListResponse {
  clusters: K8sCluster[]
  total: number
}

export interface NodeListResponse {
  nodes: K8sNode[]
  total: number
}

export interface PodListResponse {
  pods: K8sPod[]
  total: number
}

export interface NamespaceListResponse {
  namespaces: K8sNamespace[]
  total: number
}

export const kubernetesApi = {
  listClusters: (params?: { environment?: string }) =>
    apiClient.get<ClusterListResponse>('/api/v1/kubernetes/clusters', { params }),

  getCluster: (id: string) =>
    apiClient.get<K8sCluster>(`/api/v1/kubernetes/clusters/${id}`),

  getNodes: (clusterId: string) =>
    apiClient.get<NodeListResponse>(`/api/v1/kubernetes/clusters/${clusterId}/nodes`),

  getPods: (clusterId: string, params?: { namespace?: string }) =>
    apiClient.get<PodListResponse>(`/api/v1/kubernetes/clusters/${clusterId}/pods`, { params }),

  getNamespaces: (clusterId: string) =>
    apiClient.get<NamespaceListResponse>(`/api/v1/kubernetes/clusters/${clusterId}/namespaces`),
}