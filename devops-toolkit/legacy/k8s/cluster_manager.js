/**
 * K8s Cluster Manager
 * Multi-cluster Kubernetes management with kind or k3d
 *
 * Supports two providers:
 * - kind: Full Kubernetes clusters using Docker containers
 * - k3d: Lightweight k3s clusters (faster creation, less memory)
 */

const { execSync, exec } = require('child_process');
const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

class K8sClusterManager {
  constructor(options = {}) {
    this.clusters = new Map();
    this.kubeconfigDir = options.kubeconfigDir || path.join(process.env.HOME, '.kube');
    this.provider = options.provider || 'k3d'; // 'kind' or 'k3d'
    this.kindPath = options.kindPath || 'kind';
    this.k3dPath = options.k3dPath || 'k3d';
    this.kubectlPath = options.kubectlPath || 'kubectl';
  }

  /**
   * Get kubeconfig path for a cluster
   */
  getKubeconfigPath(clusterName) {
    return path.join(this.kubeconfigDir, `config-${clusterName}`);
  }

  /**
   * Check if provider is available
   */
  isProviderAvailable() {
    try {
      if (this.provider === 'k3d') {
        execSync(`${this.k3dPath} version`, { stdio: 'pipe' });
      } else {
        execSync(`${this.kindPath} version`, { stdio: 'pipe' });
      }
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Check if kind is available (for backwards compatibility)
   */
  isKindAvailable() {
    return this.isProviderAvailable() && this.provider === 'kind';
  }

  /**
   * Check if k3d is available
   */
  isK3dAvailable() {
    return this.isProviderAvailable() && this.provider === 'k3d';
  }

  /**
   * Create a cluster (kind or k3d based on provider setting)
   */
  async createCluster(clusterName, options = {}) {
    if (this.provider === 'k3d') {
      return this._createK3dCluster(clusterName, options);
    } else {
      return this._createKindCluster(clusterName, options);
    }
  }

  /**
   * Create a k3d cluster
   * k3d is faster and more lightweight than kind
   */
  async _createK3dCluster(clusterName, options = {}) {
    const { agents = 3, apiPort = 31000, k3sArgs = [] } = options;

    console.log(`[K8sManager] Creating k3d cluster ${clusterName} with ${agents} agents...`);

    try {
      // Build k3d create command
      const args = [
        'cluster', 'create', clusterName,
        '--agents', String(agents),
        '-p', `${apiPort}:6443@loadbalancer`,
        '--timeout', '120s',
        '--wait'
      ];

      // Add disabled components (traefik is disabled by default in k3d for lighter footprint)
      args.push('--k3s-arg', 'Disable=traefik@server:0');

      // Execute k3d cluster create
      execSync(`${this.k3dPath} ${args.join(' ')}`, {
        stdio: 'inherit'
      });

      // Export kubeconfig
      const kubeconfigPath = this.getKubeconfigPath(clusterName);
      execSync(`${this.k3dPath} kubeconfig get ${clusterName} > ${kubeconfigPath}`, {
        stdio: 'pipe'
      });

      // Store cluster info
      this.clusters.set(clusterName, {
        name: clusterName,
        kubeconfig: kubeconfigPath,
        provider: 'k3d',
        agents: agents,
        apiPort: apiPort,
        createdAt: new Date().toISOString(),
        status: 'running'
      });

      console.log(`[K8sManager] k3d cluster ${clusterName} created successfully`);

      return { success: true, clusterName, kubeconfigPath, provider: 'k3d' };
    } catch (error) {
      console.error(`[K8sManager] Failed to create k3d cluster ${clusterName}:`, error.message);
      return { success: false, error: error.message, provider: 'k3d' };
    }
  }

  /**
   * Create a kind cluster (original implementation)
   */
  async _createKindCluster(clusterName, options = {}) {
    const { nodes = 3, portMappings = {} } = options;

    console.log(`[K8sManager] Creating kind cluster ${clusterName} with ${nodes} nodes`);

    // Generate kind config
    const config = this._generateKindConfig(clusterName, nodes, portMappings);
    const configPath = path.join('/tmp', `kind-config-${clusterName}.yaml`);

    fs.writeFileSync(configPath, yaml.dump(config));

    try {
      // Create cluster
      execSync(`${this.kindPath} create cluster --name ${clusterName} --config ${configPath} --wait 60s`, {
        stdio: 'inherit'
      });

      // Export kubeconfig
      const kubeconfigPath = this.getKubeconfigPath(clusterName);
      execSync(`${this.kindPath} get kubeconfig --name ${clusterName} > ${kubeconfigPath}`, {
        stdio: 'pipe'
      });

      // Store cluster info
      this.clusters.set(clusterName, {
        name: clusterName,
        kubeconfig: kubeconfigPath,
        provider: 'kind',
        nodes: nodes,
        createdAt: new Date().toISOString(),
        status: 'running'
      });

      // Cleanup temp config
      fs.unlinkSync(configPath);

      return { success: true, clusterName, kubeconfigPath, provider: 'kind' };
    } catch (error) {
      console.error(`[K8sManager] Failed to create kind cluster ${clusterName}:`, error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * Generate kind cluster configuration
   */
  _generateKindConfig(clusterName, numNodes, portMappings) {
    const config = {
      kind: 'Cluster',
      apiVersion: 'kind.x-k8s.io/v1alpha4',
      nodes: [
        {
          role: 'control-plane',
          kubeadmConfigPatches: [
            'apiVersion: kubeadm.k8s.io/v1beta2\nkind: ClusterConfiguration\nmetadata:\n  name: config\nnetworking:\n  dnsDomain: cluster.local\n  podSubnet: 10.244.0.0/16\n  serviceSubnet: 10.96.0.0/12'
          ]
        }
      ]
    };

    // Add worker nodes
    for (let i = 1; i < numNodes; i++) {
      const workerNode = { role: 'worker' };

      // Add port mappings for worker nodes if specified
      if (portMappings[i]) {
        workerNode.extraPortMappings = Object.entries(portMappings[i]).map(([hostPort, containerPort]) => ({
          containerPort: hostPort,
          hostPort: containerPort,
          protocol: 'TCP'
        }));
      }

      config.nodes.push(workerNode);
    }

    return config;
  }

  /**
   * Delete a cluster
   */
  async deleteCluster(clusterName) {
    try {
      if (this.provider === 'k3d') {
        execSync(`${this.k3dPath} cluster delete ${clusterName}`, { stdio: 'pipe' });
      } else {
        execSync(`${this.kindPath} delete cluster --name ${clusterName}`, { stdio: 'pipe' });
      }

      // Remove kubeconfig
      const kubeconfigPath = this.getKubeconfigPath(clusterName);
      if (fs.existsSync(kubeconfigPath)) {
        fs.unlinkSync(kubeconfigPath);
      }

      this.clusters.delete(clusterName);

      return { success: true };
    } catch (error) {
      console.error(`[K8sManager] Failed to delete cluster ${clusterName}:`, error.message);
      return { success: false, error: error.message };
    }
  }

  /**
   * List all clusters
   */
  listClusters() {
    try {
      let output;
      if (this.provider === 'k3d') {
        output = execSync(`${this.k3dPath} cluster list`, { encoding: 'utf8' });
      } else {
        output = execSync(`${this.kindPath} get clusters`, { encoding: 'utf8' });
      }

      const lines = output.trim().split('\n').filter(line => line && !line.startsWith('NAME'));

      return lines.map(line => {
        const parts = line.split(/\s+/);
        const name = this.provider === 'k3d' ? parts[0] : parts[0];
        const info = this.clusters.get(name) || { name, status: parts[1] || 'running' };
        return info;
      });
    } catch (error) {
      return [];
    }
  }

  /**
   * Check cluster health
   */
  async healthCheck(clusterName) {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    if (!fs.existsSync(kubeconfig)) {
      return { connected: false, error: 'kubeconfig not found' };
    }

    try {
      // Check nodes
      const nodesOutput = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} get nodes -o json`,
        { encoding: 'utf8' }
      );
      const nodes = JSON.parse(nodesOutput);

      // Check all nodes are ready
      const allReady = nodes.items.every(node => {
        const condition = node.status.conditions.find(c => c.type === 'Ready');
        return condition && condition.status === 'True';
      });

      return {
        connected: true,
        ready: allReady,
        nodes: nodes.items.map(n => ({
          name: n.metadata.name,
          status: n.status.phase,
          ready: n.status.conditions.find(c => c.type === 'Ready')?.status === 'True'
        }))
      };
    } catch (error) {
      return { connected: false, error: error.message };
    }
  }

  /**
   * Health check all clusters
   */
  async healthCheckAll() {
    const results = [];

    for (const [name] of this.clusters) {
      const health = await this.healthCheck(name);
      results.push({ cluster: name, ...health });
    }

    return results;
  }

  /**
   * Get cluster metrics (CPU/Memory)
   */
  async getClusterMetrics(clusterName) {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    try {
      // Get top nodes
      const topOutput = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} top nodes --no-headers 2>/dev/null || echo "Metrics not available"`,
        { encoding: 'utf8' }
      );

      const metrics = [];
      const lines = topOutput.trim().split('\n');

      for (const line of lines) {
        if (line.includes('Metrics not available')) {
          return { available: false, message: 'Metrics server not installed' };
        }

        const parts = line.trim().split(/\s+/);
        if (parts.length >= 5) {
          metrics.push({
            name: parts[0],
            cpu: parts[1],
            cpuPercent: parts[2],
            memory: parts[3],
            memoryPercent: parts[4]
          });
        }
      }

      return { available: true, metrics };
    } catch (error) {
      return { available: false, error: error.message };
    }
  }

  /**
   * Deploy workload to cluster
   */
  async deployWorkload(clusterName, manifest) {
    const kubeconfig = this.getKubeconfigPath(clusterName);
    const manifestPath = path.join('/tmp', `manifest-${clusterName}.yaml`);

    try {
      fs.writeFileSync(manifestPath, yaml.dump(manifest));

      const result = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} apply -f ${manifestPath}`,
        { encoding: 'utf8' }
      );

      fs.unlinkSync(manifestPath);

      return { success: true, output: result };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Get workloads in cluster
   */
  async getWorkloads(clusterName, namespace = 'default') {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    try {
      const output = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} get deployments -n ${namespace} -o json`,
        { encoding: 'utf8' }
      );

      const deployments = JSON.parse(output);

      return deployments.items.map(d => ({
        name: d.metadata.name,
        namespace: d.metadata.namespace,
        replicas: d.spec.replicas,
        readyReplicas: d.status.readyReplicas || 0,
        availableReplicas: d.status.availableReplicas || 0
      }));
    } catch (error) {
      return [];
    }
  }

  /**
   * Scale workload
   */
  async scaleWorkload(clusterName, deployment, replicas, namespace = 'default') {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    try {
      execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} scale deployment ${deployment} --replicas=${replicas} -n ${namespace}`,
        { stdio: 'pipe' }
      );

      return { success: true };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Get cluster info
   */
  async getClusterInfo(clusterName) {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    try {
      const versionOutput = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} version -o json`,
        { encoding: 'utf8' }
      );

      const nodesOutput = execSync(
        `${this.kubectlPath} --kubeconfig=${kubeconfig} get nodes -o json`,
        { encoding: 'utf8' }
      );

      const version = JSON.parse(versionOutput);
      const nodes = JSON.parse(nodesOutput);

      return {
        serverVersion: version.server.gitVersion,
        kubernetesVersion: version.client.gitVersion,
        nodes: nodes.items.map(n => ({
          name: n.metadata.name,
          role: n.metadata.labels['node-role.kubernetes.io/control-plane'] ? 'control-plane' : 'worker',
          kubernetesVersion: n.status.nodeInfo.kubeletVersion
        }))
      };
    } catch (error) {
      return { error: error.message };
    }
  }

  /**
   * Collect logs from pod
   */
  async getPodLogs(clusterName, podName, namespace = 'default', options = {}) {
    const kubeconfig = this.getKubeconfigPath(clusterName);
    const { tail = 100, previous = false } = options;

    try {
      const cmd = `${this.kubectlPath} --kubeconfig=${kubeconfig} logs ${podName} -n ${namespace} --tail=${tail}${previous ? ' --previous' : ''}`;
      const logs = execSync(cmd, { encoding: 'utf8', maxBuffer: 10 * 1024 * 1024 });

      return { success: true, logs };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Execute command in pod
   */
  async execInPod(clusterName, podName, container, command, namespace = 'default') {
    const kubeconfig = this.getKubeconfigPath(clusterName);

    try {
      const cmd = `${this.kubectlPath} --kubeconfig=${kubeconfig} exec ${podName} -n ${namespace} -c ${container} -- ${command}`;
      const output = execSync(cmd, { encoding: 'utf8' });

      return { success: true, output };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Install metrics server
   */
  async installMetricsServer(clusterName) {
    const manifest = {
      apiVersion: 'v1',
      kind: 'ServiceAccount',
      metadata: { name: 'metrics-server', namespace: 'kube-system' }
    };

    const result = await this.deployWorkload(clusterName, manifest);
    if (!result.success) return result;

    // Deploy metrics-server deployment
    const metricsServerManifest = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: { name: 'metrics-server', namespace: 'kube-system' },
      spec: {
        selector: { matchLabels: { 'k8s-app': 'metrics-server' } },
        template: {
          metadata: { labels: { 'k8s-app': 'metrics-server' } },
          spec: {
            serviceAccountName: 'metrics-server',
            containers: [{
              name: 'metrics-server',
              image: 'registry.k8s.io/metrics-server/metrics-server:v0.6.4',
              args: ['--kubelet-insecure-tls']
            }]
          }
        }
      }
    };

    return this.deployWorkload(clusterName, metricsServerManifest);
  }
}

module.exports = K8sClusterManager;

module.exports = K8sClusterManager;