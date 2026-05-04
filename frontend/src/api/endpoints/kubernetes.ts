import { apiClient } from '../client'

export interface K8sCluster {
  id?: number
  name: string
  type: string
  status: string
  version?: string
  environment?: string
  nodes?: number
  pods?: number
  namespaces?: number
  created_at?: string
  kubeconfig?: string
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
  node_name?: string  // backend returns node_name
  age: string
}

export interface K8sNamespace {
  name: string
  status: string
  labels?: Record<string, string>
}

export interface K8sHealth {
  status: string
  version?: string
  components?: {
    name: string
    status: string
  }[]
}

export interface K8sMetrics {
  cpu: {
    used: number
    total: number
    percentage: number
  }
  memory: {
    used: number
    total: number
    percentage: number
  }
  pods: {
    running: number
    total: number
  }
}

export interface CreateClusterRequest {
  name: string
  type: string
  environment?: string
  kubeconfig: string
  version?: string
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

export interface K8sPodLogsResponse {
  logs: string[]
  backend: string
  count: number
}

export const kubernetesApi = {
  listClusters: () =>
    apiClient.get<K8sApiResponse<K8sCluster[]>>('/api/k8s/clusters'),

  createCluster: (data: CreateClusterRequest) =>
    apiClient.post<K8sCluster>('/api/k8s/clusters', data),

  deleteCluster: (name: string) =>
    apiClient.delete(`/api/k8s/clusters/${name}`),

  getCluster: (name: string) =>
    apiClient.get<K8sApiResponse<K8sCluster>>(`/api/k8s/clusters/${name}`),

  getClusterHealth: (name: string) =>
    apiClient.get<K8sHealth>(`/api/k8s/clusters/${name}/health`),

  getClusterMetrics: (name: string) =>
    apiClient.get<K8sMetrics>(`/api/k8s/clusters/${name}/metrics`),

  getNodes: (clusterName: string) =>
    apiClient.get<K8sApiResponse<K8sNode[]>>(`/api/k8s/clusters/${clusterName}/nodes`),

  getPods: (clusterName: string, params?: { namespace?: string }) =>
    apiClient.get<K8sApiResponse<K8sPod[]>>(`/api/k8s/clusters/${clusterName}/pods`, { params }),

  getPodLogsHistorical: (clusterName: string, namespace: string, podName: string, params?: { start?: string; end?: string; limit?: string }) =>
    apiClient.get<K8sPodLogsResponse>(
      `/api/k8s/clusters/${clusterName}/namespaces/${namespace}/pods/${podName}/logs/historical`,
      { params }
    ),

  getNamespaces: (clusterName: string) =>
    apiClient.get<K8sApiResponse<K8sNamespace[]>>(`/api/k8s/clusters/${clusterName}/namespaces`),
}