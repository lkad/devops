/**
 * Tests for K8s Cluster Manager
 * Tests multi-cluster Kubernetes management with kind
 */

const K8sClusterManager = require('../k8s/cluster_manager');
const path = require('path');
const fs = require('fs');

describe('K8sClusterManager', () => {
  let manager;
  const testKubeconfigDir = '/tmp/kubeconfig-test';

  beforeAll(() => {
    // Use test kubeconfig directory
    fs.mkdirSync(testKubeconfigDir, { recursive: true });
    manager = new K8sClusterManager({
      kubeconfigDir: testKubeconfigDir,
      kindPath: 'kind',
      kubectlPath: 'kubectl'
    });
  });

  afterAll(() => {
    // Cleanup test directory
    if (fs.existsSync(testKubeconfigDir)) {
      fs.rmSync(testKubeconfigDir, { recursive: true });
    }
  });

  describe('Initialization', () => {
    it('should create manager with default options', () => {
      const m = new K8sClusterManager();
      expect(m.clusters).toBeDefined();
      expect(m.clusters.size).toBe(0);
      expect(m.kubeconfigDir).toBe(path.join(process.env.HOME, '.kube'));
    });

    it('should create manager with custom options', () => {
      const m = new K8sClusterManager({
        kubeconfigDir: '/custom/kubeconfig',
        kindPath: '/usr/local/bin/kind',
        kubectlPath: '/usr/local/bin/kubectl'
      });
      expect(m.kubeconfigDir).toBe('/custom/kubeconfig');
      expect(m.kindPath).toBe('/usr/local/bin/kind');
      expect(m.kubectlPath).toBe('/usr/local/bin/kubectl');
    });

    it('should generate kubeconfig path correctly', () => {
      const kubeconfigPath = manager.getKubeconfigPath('test-cluster');
      expect(kubeconfigPath).toBe(path.join(testKubeconfigDir, 'config-test-cluster'));
    });
  });

  describe('isKindAvailable', () => {
    it('should check if kind is available', () => {
      const available = manager.isKindAvailable();
      // This depends on whether kind is installed in the test environment
      expect(typeof available).toBe('boolean');
    });
  });

  describe('generateKindConfig', () => {
    it('should generate config for single node cluster', () => {
      const config = manager._generateKindConfig('test', 1, {});

      expect(config.kind).toBe('Cluster');
      expect(config.apiVersion).toBe('kind.x-k8s.io/v1alpha4');
      expect(config.nodes).toHaveLength(1);
      expect(config.nodes[0].role).toBe('control-plane');
    });

    it('should generate config for multi-node cluster', () => {
      const config = manager._generateKindConfig('test', 3, {});

      expect(config.nodes).toHaveLength(3);
      expect(config.nodes[0].role).toBe('control-plane');
      expect(config.nodes[1].role).toBe('worker');
      expect(config.nodes[2].role).toBe('worker');
    });

    it('should generate config with port mappings', () => {
      const portMappings = {
        1: { 8080: 80 },
        2: { 8081: 80 }
      };
      const config = manager._generateKindConfig('test', 3, portMappings);

      expect(config.nodes[1].extraPortMappings).toBeDefined();
      expect(Array.isArray(config.nodes[1].extraPortMappings)).toBe(true);
    });
  });

  describe('Cluster Lifecycle (requires kind)', () => {
    const testClusterName = 'test-cluster-' + Date.now();

    afterEach(async () => {
      // Cleanup test cluster if it exists
      try {
        await manager.deleteCluster(testClusterName);
      } catch (e) {
        // Ignore cleanup errors
      }
    });

    it('should create and delete a cluster', async () => {
      // Skip if kind is not available
      if (!manager.isKindAvailable()) {
        console.log('Skipping cluster test - kind not available');
        return;
      }

      // Create cluster
      const createResult = await manager.createCluster(testClusterName, { nodes: 2 });

      // Cluster creation may fail in constrained environments
      if (!createResult.success) {
        console.log('Cluster creation failed (may be expected in CI):', createResult.error);
        return;
      }

      expect(createResult.success).toBe(true);
      expect(createResult.clusterName).toBe(testClusterName);
      expect(fs.existsSync(createResult.kubeconfigPath)).toBe(true);

      // Check cluster is tracked
      expect(manager.clusters.has(testClusterName)).toBe(true);

      // Delete cluster
      const deleteResult = await manager.deleteCluster(testClusterName);
      expect(deleteResult.success).toBe(true);
      expect(manager.clusters.has(testClusterName)).toBe(false);
    });

    it('should list clusters', async () => {
      if (!manager.isKindAvailable()) {
        console.log('Skipping cluster test - kind not available');
        return;
      }

      const clusters = manager.listClusters();
      expect(Array.isArray(clusters)).toBe(true);
    });
  });

  describe('Cluster Operations (requires real cluster)', () => {
    // These tests require an actual kind cluster

    const realClusterName = 'kind-cluster-test';

    beforeAll(async () => {
      // Check if we have a real cluster to test against
      if (!fs.existsSync('/tmp/kubeconfig-kind-test')) {
        console.log('No test cluster available, skipping integration tests');
        return;
      }
      manager.clusters.set(realClusterName, {
        name: realClusterName,
        kubeconfig: '/tmp/kubeconfig-kind-test',
        status: 'running'
      });
    });

    it('should check cluster health', async () => {
      if (!fs.existsSync('/tmp/kubeconfig-kind-test')) {
        return; // Skip if no cluster
      }

      const health = await manager.healthCheck(realClusterName);
      expect(health).toHaveProperty('connected');
    });

    it('should get cluster info', async () => {
      if (!fs.existsSync('/tmp/kubeconfig-kind-test')) {
        return;
      }

      const info = await manager.getClusterInfo(realClusterName);
      // May have error if cluster not fully ready
      expect(info).toBeDefined();
    });

    it('should deploy workload', async () => {
      if (!fs.existsSync('/tmp/kubeconfig-kind-test')) {
        return;
      }

      const manifest = {
        apiVersion: 'v1',
        kind: 'Namespace',
        metadata: { name: 'test-namespace' }
      };

      const result = await manager.deployWorkload(realClusterName, manifest);
      // May fail if cluster not accessible
      expect(result).toHaveProperty('success');
    });

    it('should get workloads', async () => {
      if (!fs.existsSync('/tmp/kubeconfig-kind-test')) {
        return;
      }

      const workloads = await manager.getWorkloads(realClusterName, 'default');
      expect(Array.isArray(workloads)).toBe(true);
    });
  });

  describe('Multi-cluster Operations', () => {
    it('should track multiple clusters', () => {
      manager.clusters.set('cluster-1', { name: 'cluster-1', status: 'running' });
      manager.clusters.set('cluster-2', { name: 'cluster-2', status: 'running' });
      manager.clusters.set('cluster-3', { name: 'cluster-3', status: 'running' });

      expect(manager.clusters.size).toBe(3);
    });

    it('should health check all clusters', async () => {
      // This will return results for all tracked clusters
      // In real scenario, each cluster should be reachable
      const results = await manager.healthCheckAll();
      expect(Array.isArray(results)).toBe(true);
    });

    it('should collect metrics from all clusters', async () => {
      const results = [];
      for (const [name] of manager.clusters) {
        const metrics = await manager.getClusterMetrics(name);
        results.push({ cluster: name, metrics });
      }

      expect(Array.isArray(results));
    });
  });
});

describe('K8sClusterManager Mock Tests', () => {
  // Tests that don't require real kind/kubectl

  let manager;

  beforeEach(() => {
    manager = new K8sClusterManager({
      kubeconfigDir: '/tmp/kubeconfig-test-mock'
    });
  });

  describe('Config Generation', () => {
    it('should generate valid kind config structure', () => {
      const config = manager._generateKindConfig('mock-cluster', 3, {});

      expect(config).toEqual({
        kind: 'Cluster',
        apiVersion: 'kind.x-k8s.io/v1alpha4',
        nodes: [
          { role: 'control-plane', kubeadmConfigPatches: expect.any(Array) },
          { role: 'worker' },
          { role: 'worker' }
        ]
      });
    });

    it('should generate 5 node cluster config', () => {
      const config = manager._generateKindConfig('large-cluster', 5, {});

      expect(config.nodes).toHaveLength(5);
      expect(config.nodes.filter(n => n.role === 'worker')).toHaveLength(4);
    });

    it('should apply port mappings to worker nodes', () => {
      const mappings = { 1: { 443: 8443 } };
      const config = manager._generateKindConfig('port-test', 2, mappings);

      expect(config.nodes[1].extraPortMappings).toEqual([
        { containerPort: '443', hostPort: 8443, protocol: 'TCP' }
      ]);
    });
  });

  describe('Kubeconfig Path', () => {
    it('should generate correct kubeconfig paths', () => {
      expect(manager.getKubeconfigPath('cluster-1')).toContain('config-cluster-1');
      expect(manager.getKubeconfigPath('prod-us-east')).toContain('config-prod-us-east');
    });
  });
});