/**
 * K8s API Mock for Testing
 * Provides mock responses for kubectl commands without requiring a real cluster
 */

// Mock K8s API responses for different clusters
const mockK8sApi = {
  'cluster-1': {
    nodes: [
      {
        metadata: { name: 'cluster-1-control-plane', labels: { 'node-role.kubernetes.io/control-plane': '' } },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.28.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      },
      {
        metadata: { name: 'cluster-1-worker', labels: {} },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.28.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      }
    ],
    deployments: [
      { metadata: { name: 'nginx', namespace: 'default' }, spec: { replicas: 2 }, status: { readyReplicas: 2, availableReplicas: 2 } }
    ],
    version: { server: { gitVersion: 'v1.28.0' }, client: { gitVersion: 'v1.28.0' } }
  },
  'cluster-2': {
    nodes: [
      {
        metadata: { name: 'cluster-2-control-plane', labels: { 'node-role.kubernetes.io/control-plane': '' } },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.28.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      },
      {
        metadata: { name: 'cluster-2-worker', labels: {} },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.28.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      },
      {
        metadata: { name: 'cluster-2-worker-2', labels: {} },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.28.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      }
    ],
    deployments: [
      { metadata: { name: 'redis', namespace: 'cache' }, spec: { replicas: 3 }, status: { readyReplicas: 3, availableReplicas: 3 } }
    ],
    version: { server: { gitVersion: 'v1.28.0' }, client: { gitVersion: 'v1.28.0' } }
  },
  'cluster-3': {
    nodes: [
      {
        metadata: { name: 'cluster-3-control-plane', labels: { 'node-role.kubernetes.io/control-plane': '' } },
        status: {
          phase: 'Ready',
          nodeInfo: { kubeletVersion: 'v1.27.0' },
          conditions: [{ type: 'Ready', status: 'True' }]
        }
      }
    ],
    deployments: [],
    version: { server: { gitVersion: 'v1.27.0' }, client: { gitVersion: 'v1.27.0' } }
  }
};

/**
 * Create mock kubectl responses for a cluster
 */
function createKubectlMock(clusterName) {
  const clusterData = mockK8sApi[clusterName] || mockK8sApi['cluster-1'];

  return {
    // Mock kubectl get nodes
    getNodes: {
      items: clusterData.nodes
    },

    // Mock kubectl get deployments
    getDeployments: {
      items: clusterData.deployments
    },

    // Mock kubectl version
    version: clusterData.version,

    // Mock kubectl top nodes (requires metrics-server)
    topNodes: clusterData.nodes.map(n => ({
      name: n.metadata.name,
      cpu: '100m',
      cpuPercent: '10%',
      memory: '256Mi',
      memoryPercent: '25%'
    })).join('\n'),

    // Mock kubectl get pods
    getPods: {
      items: [
        {
          metadata: { name: 'nginx-pod-1', namespace: 'default' },
          status: { phase: 'Running', podIP: '10.244.0.10' }
        }
      ]
    }
  };
}

/**
 * Mock execSync for kubectl commands
 * Returns mock data instead of executing real commands
 */
function mockKubectlExec(command) {
  const mocks = createKubectlMock('cluster-1');

  if (command.includes('get nodes') && command.includes('-o json')) {
    return JSON.stringify({ items: mocks.getNodes.items });
  }

  if (command.includes('get deployments') && command.includes('-o json')) {
    return JSON.stringify({ items: mocks.getDeployments.items });
  }

  if (command.includes('version') && command.includes('-o json')) {
    return JSON.stringify(mocks.version);
  }

  if (command.includes('top nodes')) {
    return mocks.topNodes;
  }

  if (command.includes('get pods') && command.includes('-o json')) {
    return JSON.stringify({ items: mocks.getPods.items });
  }

  if (command.includes('apply')) {
    return 'deployment.apps/nginx created';
  }

  if (command.includes('scale')) {
    return 'deployment.apps/nginx scaled';
  }

  if (command.includes('logs')) {
    return 'Mock log output from pod';
  }

  if (command.includes('exec')) {
    return 'Mock command output';
  }

  return '';
}

/**
 * Setup all kubectl mocks for multi-cluster testing
 */
function setupMultiClusterMocks() {
  const clusters = ['cluster-1', 'cluster-2', 'cluster-3'];

  clusters.forEach(clusterName => {
    const data = mockK8sApi[clusterName];

    // Create mock for each cluster's kubeconfig
    // In real scenarios, these would be different files
  });

  return { clusters, totalNodes: clusters.reduce((sum, c) => sum + mockK8sApi[c].nodes.length, 0) };
}

/**
 * Generate test data for multi-cluster scenarios
 */
function generateMultiClusterTestData() {
  return {
    clusters: [
      {
        name: 'cluster-1',
        region: 'us-east-1',
        environment: 'production',
        nodes: 2,
        status: 'healthy',
        workloads: 5,
        cpuUsage: '45%',
        memoryUsage: '60%'
      },
      {
        name: 'cluster-2',
        region: 'us-west-2',
        environment: 'staging',
        nodes: 3,
        status: 'healthy',
        workloads: 12,
        cpuUsage: '30%',
        memoryUsage: '45%'
      },
      {
        name: 'cluster-3',
        region: 'eu-west-1',
        environment: 'development',
        nodes: 1,
        status: 'healthy',
        workloads: 3,
        cpuUsage: '15%',
        memoryUsage: '25%'
      }
    ],
    totalNodes: 6,
    totalWorkloads: 20,
    averageCpuUsage: '30%',
    averageMemoryUsage: '43%'
  };
}

module.exports = {
  mockK8sApi,
  createKubectlMock,
  mockKubectlExec,
  setupMultiClusterMocks,
  generateMultiClusterTestData
};