import { apiClient } from '../client'

export interface K8sCluster {
  name: string
  type: string
  status: string
  version?: string
  environment?: string
  nodes?: number
  pods?: number
  namespaces?: number
  created_at?: string
}

export interface K8sNode {
  name: string
  status: string
  role: string
  age: string
  version?: string
}

export interface K8sPod {
  name: string
  namespace: string
  status: string
  node: string
  age: string
}

export interface K8sNamespace {
  name: string
  status: string
  labels?: Record<string, string>
}

export interface K8sApiResponse<T> {
  data: T
  pagination?: {
    total: number
    limit: number
    offset: number
    has_more: boolean
  }
}

export const kubernetesApi = {
  listClusters: () =>
    apiClient.get<K8sApiResponse<K8sCluster[]>>('/api/k8s/clusters'),

  getCluster: (name: string) =>
    apiClient.get<K8sApiResponse<K8sCluster>>(`/api/k8s/clusters/${name}`),

  getNodes: (clusterName: string) =>
    apiClient.get<K8sApiResponse<K8sNode[]>>(`/api/k8s/clusters/${clusterName}/nodes`),

  getPods: (clusterName: string, params?: { namespace?: string }) =>
    apiClient.get<K8sApiResponse<K8sPod[]>>(`/api/k8s/clusters/${clusterName}/pods`, { params }),

  getNamespaces: (clusterName: string) =>
    apiClient.get<K8sApiResponse<K8sNamespace[]>>(`/api/k8s/clusters/${clusterName}/namespaces`),
}