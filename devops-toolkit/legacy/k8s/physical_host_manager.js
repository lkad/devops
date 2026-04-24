/**
 * Physical Host Manager
 * Manages SSH connections, state monitoring, and metrics collection for physical hosts
 */

const EventEmitter = require('events');

// SSH client - can be injected for testing
let SSHClient = null;
try {
  SSHClient = require('ssh2').Client;
} catch (e) {
  // ssh2 not installed - will use mock mode
}

class PhysicalHostManager extends EventEmitter {
  constructor(options = {}) {
    super();
    this.hosts = new Map();
    this.connectionPool = new Map();
    this.poolSize = options.poolSize || 10;
    this.heartbeatInterval = options.heartbeatInterval || 30000; // 30s
    this.commandTimeout = options.commandTimeout || 10000;
    this.maxRetries = options.maxRetries || 3;
    this.retryDelay = options.retryDelay || 5000;

    this.heartbeatTimers = new Map();
  }

  // ============ Host Registration ============

  /**
   * Register a physical host
   */
  registerHost(hostInfo) {
    const { id, hostname, ip, port = 22, username, authMethod = 'password', credentials = {} } = hostInfo;

    const host = {
      id,
      hostname,
      ip,
      port,
      username,
      authMethod, // 'password' | 'key' | 'agent'
      credentials,
      state: 'offline',          // 本地状态: online | offline (基于 lastAgentUpdate)
      lastHeartbeat: null,       // SSH探测时间
      lastAgentUpdate: null,     // 数据新鲜度时间戳 (30s 内无数据 → offline)
      dataStatus: 'unavailable', // 数据状态: fresh | stale | unavailable
      connectionError: null,
      metrics: {                 // 本地缓存 (最近数据)
        cpu: null,
        memory: null,
        disk: null,
        uptime: null,
        collectedAt: null
      },
      services: new Map(),
      registeredAt: new Date().toISOString(),
      metadata: {                // 数据源配置
        influxdbHost: hostInfo.influxdbHost || null,
        prometheusJob: hostInfo.prometheusJob || null,
        dataSource: hostInfo.dataSource || 'telegraf'  // 'telegraf' | 'prometheus'
      }
    };

    this.hosts.set(id, host);
    this.emit('host:registered', host);

    return host;
  }

  /**
   * Remove a host
   */
  removeHost(hostId) {
    const host = this.hosts.get(hostId);
    if (!host) return false;

    // Close any open connections
    this.closeConnection(hostId);

    // Stop heartbeat
    if (this.heartbeatTimers.has(hostId)) {
      clearInterval(this.heartbeatTimers.get(hostId));
      this.heartbeatTimers.delete(hostId);
    }

    this.hosts.delete(hostId);
    this.emit('host:removed', { id: hostId });

    return true;
  }

  // ============ SSH Connection Management ============

  /**
   * Get or create SSH connection from pool
   */
  async getConnection(hostId) {
    const host = this.hosts.get(hostId);
    if (!host) {
      throw new Error(`Host ${hostId} not found`);
    }

    // Return existing connection if alive
    if (this.connectionPool.has(hostId)) {
      const conn = this.connectionPool.get(hostId);
      if (conn && conn._channel && !conn._channel_closed) {
        return conn;
      }
    }

    // Create new connection
    const conn = await this._createConnection(host);
    this.connectionPool.set(hostId, conn);
    host.lastConnect = new Date().toISOString();

    return conn;
  }

  /**
   * Create new SSH connection
   */
  _createConnection(host) {
    if (!SSHClient) {
      // Mock mode - no ssh2 installed
      return Promise.reject(new Error('SSH client not available (ssh2 module not installed)'));
    }

    return new Promise((resolve, reject) => {
      const conn = new SSHClient();

      const connectionConfig = {
        host: host.ip,
        port: host.port,
        username: host.username,
        readyTimeout: this.commandTimeout,
        keepaliveInterval: 10000
      };

      // Auth method
      if (host.authMethod === 'password') {
        connectionConfig.password = host.credentials.password;
      } else if (host.authMethod === 'key') {
        connectionConfig.privateKey = host.credentials.privateKey;
        connectionConfig.passphrase = host.credentials.passphrase;
      }

      conn.on('ready', () => {
        this.emit('connection:opened', { hostId: host.id });
        resolve(conn);
      });

      conn.on('error', (err) => {
        this.emit('connection:error', { hostId: host.id, error: err.message });
        reject(err);
      });

      conn.on('close', () => {
        this.emit('connection:closed', { hostId: host.id });
        const hostRef = this.hosts.get(host.id);
        if (hostRef) {
          hostRef.state = 'offline';
        }
        this.connectionPool.delete(host.id);
      });

      conn.connect(connectionConfig);
    });
  }

  /**
   * Close connection for a host
   */
  closeConnection(hostId) {
    if (this.connectionPool.has(hostId)) {
      const conn = this.connectionPool.get(hostId);
      conn.end();
      this.connectionPool.delete(hostId);
    }
  }

  /**
   * Execute command on host
   */
  async executeCommand(hostId, command, options = {}) {
    const timeout = options.timeout || this.commandTimeout;

    let conn;
    try {
      conn = await this.getConnection(hostId);

      return new Promise((resolve, reject) => {
        const timeoutId = setTimeout(() => {
          reject(new Error(`Command timeout after ${timeout}ms`));
        }, timeout);

        conn.exec(command, (err, stream) => {
          if (err) {
            clearTimeout(timeoutId);
            reject(err);
            return;
          }

          let stdout = '';
          let stderr = '';

          stream.on('close', (code) => {
            clearTimeout(timeoutId);
            resolve({ stdout, stderr, code, command });
          });

          stream.on('data', (data) => { stdout += data.toString(); });
          stream.stderr.on('data', (data) => { stderr += data.toString(); });
        });
      });
    } catch (error) {
      throw error;
    }
  }

  // ============ State Management ============

  /**
   * Start heartbeat monitoring for a host
   */
  startHeartbeat(hostId) {
    if (this.heartbeatTimers.has(hostId)) {
      return; // Already running
    }

    const timer = setInterval(async () => {
      await this._doHeartbeat(hostId);
    }, this.heartbeatInterval);

    this.heartbeatTimers.set(hostId, timer);

    // Do initial heartbeat
    this._doHeartbeat(hostId);
  }

  /**
   * Stop heartbeat monitoring
   */
  stopHeartbeat(hostId) {
    if (this.heartbeatTimers.has(hostId)) {
      clearInterval(this.heartbeatTimers.get(hostId));
      this.heartbeatTimers.delete(hostId);
    }
  }

  /**
   * Perform heartbeat check
   */
  async _doHeartbeat(hostId) {
    const host = this.hosts.get(hostId);
    if (!host) return;

    try {
      // Try to execute a simple command
      await this.executeCommand(hostId, 'echo ok', { timeout: 5000 });

      host.state = 'online';
      host.lastHeartbeat = new Date().toISOString();
      host.connectionError = null;

      this.emit('host:online', { hostId, lastHeartbeat: host.lastHeartbeat });

    } catch (error) {
      host.state = 'offline';
      host.connectionError = error.message;

      this.emit('host:offline', { hostId, error: error.message });
    }
  }

  /**
   * Batch check all hosts
   */
  async checkAllHosts() {
    const results = [];

    for (const [hostId] of this.hosts) {
      try {
        await this._doHeartbeat(hostId);
        const host = this.hosts.get(hostId);
        results.push({ hostId, state: host.state, lastHeartbeat: host.lastHeartbeat });
      } catch (error) {
        results.push({ hostId, state: 'error', error: error.message });
      }
    }

    return results;
  }

  // ============ Metrics Collection ============

  /**
   * Collect all metrics from a host
   */
  async collectMetrics(hostId) {
    const host = this.hosts.get(hostId);
    if (!host) throw new Error(`Host ${hostId} not found`);

    if (host.state !== 'online') {
      return { ...host.metrics, error: 'Host is offline' };
    }

    try {
      // Parallel collection for speed
      const [cpu, memory, disk, uptime] = await Promise.all([
        this._collectCpuMetrics(hostId),
        this._collectMemoryMetrics(hostId),
        this._collectDiskMetrics(hostId),
        this._collectUptime(hostId)
      ]);

      host.metrics = { cpu, memory, disk, uptime, collectedAt: new Date().toISOString() };

      this.emit('metrics:collected', { hostId, metrics: host.metrics });

      return host.metrics;
    } catch (error) {
      this.emit('metrics:error', { hostId, error: error.message });
      throw error;
    }
  }

  /**
   * Collect CPU metrics
   */
  async _collectCpuMetrics(hostId) {
    // Linux: top + mpstat style
    const result = await this.executeCommand(hostId,
      `cat /proc/stat | head -1 && cat /proc/loadavg`);

    const lines = result.stdout.trim().split('\n');

    // Parse /proc/stat
    const statMatch = lines[0].match(/cpu\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)/);
    const loadavg = lines[1]?.split(' ') || [];

    if (statMatch) {
      const user = parseInt(statMatch[1]);
      const nice = parseInt(statMatch[2]);
      const system = parseInt(statMatch[3]);
      const idle = parseInt(statMatch[4]);
      const total = user + nice + system + idle;

      return {
        usage: parseFloat(loadavg[0] || '0'),
        cores: require('os').cpus().length,
        user,
        system,
        idle,
        total
      };
    }

    return { usage: parseFloat(loadavg[0] || '0'), cores: 1 };
  }

  /**
   * Collect Memory metrics
   */
  async _collectMemoryMetrics(hostId) {
    const result = await this.executeCommand(hostId,
      `cat /proc/meminfo | grep -E "^(MemTotal|MemFree|MemAvailable|Cached|SwapTotal|SwapFree):"`);

    const lines = result.stdout.trim().split('\n');
    const mem = {};

    for (const line of lines) {
      const [key, value] = line.split(':');
      const num = parseInt(value?.trim() || '0');
      mem[key] = Math.round(num / 1024); // Convert to MB
    }

    const total = mem.MemTotal || 0;
    const free = mem.MemFree || 0;
    const available = mem.MemAvailable || free;
    const used = total - available;
    const swapTotal = mem.SwapTotal || 0;
    const swapFree = mem.SwapFree || 0;

    return {
      total,
      free,
      available,
      used,
      usagePercent: Math.round((used / total) * 100),
      swap: {
        total: swapTotal,
        free: swapFree,
        used: swapTotal - swapFree
      }
    };
  }

  /**
   * Collect Disk metrics
   */
  async _collectDiskMetrics(hostId) {
    const result = await this.executeCommand(hostId,
      `df -h | grep -E "^/dev" | awk '{print $1":"$2":"$3":"$4":"$5}'`);

    const disks = [];
    const lines = result.stdout.trim().split('\n');

    for (const line of lines) {
      if (!line) continue;
      const [device, size, used, available, percent] = line.split(':');
      disks.push({
        device,
        size,
        used,
        available,
        percent: parseInt(percent)
      });
    }

    // Also get IO stats
    const ioResult = await this.executeCommand(hostId,
      `cat /proc/diskstats | head -4`);

    return { disks, ioStats: ioResult.stdout };
  }

  /**
   * Collect Uptime
   */
  async _collectUptime(hostId) {
    const result = await this.executeCommand(hostId, `cat /proc/uptime`);
    const [seconds] = result.stdout.trim().split(' ');

    const uptime = parseFloat(seconds);
    const days = Math.floor(uptime / 86400);
    const hours = Math.floor((uptime % 86400) / 3600);
    const minutes = Math.floor((uptime % 3600) / 60);

    return {
      seconds: uptime,
      formatted: `${days}d ${hours}h ${minutes}m`,
      days,
      hours,
      minutes
    };
  }

  // ============ Service Monitoring ============

  /**
   * Check service status
   */
  async checkService(hostId, serviceName) {
    try {
      // Try systemctl first (systemd)
      const result = await this.executeCommand(hostId,
        `systemctl is-active ${serviceName} 2>/dev/null || service ${serviceName} status 2>/dev/null || pgrep -x ${serviceName}`);

      const isActive = result.stdout.trim().includes('active') || result.code === 0;

      return {
        name: serviceName,
        status: isActive ? 'running' : 'stopped',
        checkedAt: new Date().toISOString()
      };
    } catch (error) {
      return {
        name: serviceName,
        status: 'unknown',
        error: error.message
      };
    }
  }

  /**
   * Start a service
   */
  async startService(hostId, serviceName) {
    await this.executeCommand(hostId,
      `sudo systemctl start ${serviceName} 2>/dev/null || sudo service ${serviceName} start`);

    return { success: true, service: serviceName, action: 'started' };
  }

  /**
   * Stop a service
   */
  async stopService(hostId, serviceName) {
    await this.executeCommand(hostId,
      `sudo systemctl stop ${serviceName} 2>/dev/null || sudo service ${serviceName} stop`);

    return { success: true, service: serviceName, action: 'stopped' };
  }

  /**
   * Restart a service
   */
  async restartService(hostId, serviceName) {
    await this.executeCommand(hostId,
      `sudo systemctl restart ${serviceName} 2>/dev/null || sudo service ${serviceName} restart`);

    return { success: true, service: serviceName, action: 'restarted' };
  }

  // ============ Configuration Push ============

  /**
   * Push configuration file to host
   */
  async pushConfig(hostId, remotePath, content, options = {}) {
    const tempPath = `/tmp/push_config_${Date.now()}`;

    try {
      // Write to temp file first (we can only exec, not sftp without library)
      const encoded = Buffer.from(content).toString('base64');
      await this.executeCommand(hostId,
        `echo "${encoded}" | base64 -d > ${tempPath}`);

      // Move to final location with optional backup
      if (options.backup && options.backupSuffix === undefined) {
        options.backupSuffix = `.bak.${Date.now()}`;
      }

      if (options.backup) {
        await this.executeCommand(hostId,
          `sudo mv ${remotePath} ${remotePath}${options.backupSuffix} 2>/dev/null || true`);
      }

      // Copy to final location
      await this.executeCommand(hostId,
        `sudo mv ${tempPath} ${remotePath} && sudo chmod ${options.mode || '644'} ${remotePath}`);

      this.emit('config:push', { hostId, remotePath, success: true });

      return { success: true, remotePath };
    } catch (error) {
      this.emit('config:push', { hostId, remotePath, success: false, error: error.message });
      throw error;
    }
  }

  // ============ Agent Push (Telegraf) ============

  /**
   * Receive metrics pushed from Telegraf agent
   * Telegraf sends HTTP POST to /api/telegraf/receive with metrics data
   */
  receiveAgentMetrics(hostId, metricsData) {
    const host = this.hosts.get(hostId);
    if (!host) {
      return { success: false, error: 'Host not found' };
    }

    // Update host state to online and record timestamp
    host.state = 'online';
    host.lastAgentUpdate = new Date().toISOString();
    host.dataStatus = 'fresh';  // 数据新鲜

    // Update metrics cache
    host.metrics = {
      cpu: metricsData.cpu || null,
      memory: metricsData.memory || null,
      disk: metricsData.disk || null,
      uptime: metricsData.uptime || null,
      collectedAt: metricsData.collectedAt || new Date().toISOString()
    };

    // Update services status if provided
    if (metricsData.services) {
      for (const [serviceName, serviceData] of Object.entries(metricsData.services)) {
        host.services.set(serviceName, {
          name: serviceName,
          status: serviceData.status || 'unknown',
          uptime: serviceData.uptime,
          lastCheck: serviceData.lastCheck || new Date().toISOString()
        });
      }
    }

    this.emit('agent:metrics', { hostId, metrics: host.metrics });

    return { success: true, hostId, state: host.state, dataStatus: host.dataStatus };
  }

  // ============ Prometheus remote_write ============

  /**
   * Receive metrics from Prometheus remote_write
   * Prometheus sends a different format than Telegraf
   */
  receivePrometheusMetrics(body) {
    // Prometheus remote_write format: { remote_write_timeseries: [...] } or similar
    // Or it could be Prometheus exposition format converted by the agent
    const ts = body.timeseries || body.ts || [];
    if (!Array.isArray(ts) || ts.length === 0) {
      return { success: true, processed: 0 };
    }

    let processed = 0;
    for (const t of ts) {
      const hostId = t.labels?.host_id || t.labels?.hostname;
      if (!hostId) continue;

      const host = this.hosts.get(hostId);
      if (!host) continue;

      host.state = 'online';
      host.lastAgentUpdate = new Date().toISOString();

      // Parse samples from Prometheus format
      if (t.samples) {
        for (const sample of t.samples) {
          const metricName = sample.metric || sample.name;
          const value = sample.value;

          if (metricName === 'cpu_usage' || metricName === 'cpu_usage_seconds_total') {
            host.metrics.cpu = host.metrics.cpu || {};
            host.metrics.cpu.usage = value;
          } else if (metricName === 'memory_usage_bytes' || metricName === 'memory_total_bytes') {
            host.metrics.memory = host.metrics.memory || {};
            if (metricName === 'memory_total_bytes') {
              host.metrics.memory.total = Math.round(value / (1024 * 1024)); // bytes to MB
            } else {
              host.metrics.memory.used = Math.round(value / (1024 * 1024));
            }
          } else if (metricName === 'uptime_seconds') {
            host.metrics.uptime = {
              value: Math.floor(value),
              formatted: this._formatUptime(Math.floor(value))
            };
          }
        }
      }

      host.metrics.collectedAt = new Date().toISOString();
      processed++;
      this.emit('agent:metrics', { hostId, metrics: host.metrics });
    }

    return { success: true, processed };
  }

  /**
   * Get metrics in Prometheus text format (Exporter mode)
   */
  getPrometheusMetrics() {
    const lines = [];
    const timestamp = Date.now();

    for (const [hostId, host] of this.hosts) {
      // Freshness check
      let state = host.state;
      if (host.lastAgentUpdate) {
        const age = Date.now() - new Date(host.lastAgentUpdate).getTime();
        if (age > 30000) state = 'offline';
      }

      const labels = `host_id="${hostId}",hostname="${host.hostname}",ip="${host.ip}"`;

      // CPU metrics
      if (host.metrics?.cpu) {
        lines.push(`# TYPE cpu_usage gauge`);
        lines.push(`cpu_usage{${labels}} ${host.metrics.cpu.usage || 0}`);
      }

      // Memory metrics
      if (host.metrics?.memory) {
        lines.push(`# TYPE memory_usage_bytes gauge`);
        lines.push(`memory_usage_bytes{${labels}} ${(host.metrics.memory.used || 0) * 1024 * 1024}`);
        lines.push(`memory_total_bytes{${labels}} ${(host.metrics.memory.total || 0) * 1024 * 1024}`);
      }

      // Uptime
      if (host.metrics?.uptime?.value !== undefined) {
        lines.push(`# TYPE uptime_seconds gauge`);
        lines.push(`uptime_seconds{${labels}} ${host.metrics.uptime.value}`);
      }

      // State
      lines.push(`# TYPE host_state gauge`);
      lines.push(`host_state{${labels}} ${state === 'online' ? 1 : 0}`);

      // Last update
      if (host.lastAgentUpdate) {
        lines.push(`# TYPE host_last_update_timestamp gauge`);
        lines.push(`host_last_update_timestamp{${labels}} ${new Date(host.lastAgentUpdate).getTime() / 1000}`);
      }
    }

    // Add info metric
    lines.push(`# TYPE devops_physical_host_info gauge`);
    lines.push(`devops_physical_host_info{service="devops-toolkit"} 1`);

    return lines.join('\n') + '\n';
  }

  /**
   * Format uptime seconds to human readable
   */
  _formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return `${days}d ${hours}h ${mins}m`;
  }

  // ============ Getters ============

  /**
   * Get host by ID with freshness check for agent-push data
   * If agent push is active (lastAgentUpdate exists), check if data is stale (>30s)
   */
  getHost(hostId) {
    const host = this.hosts.get(hostId);
    if (!host) return undefined;

    // Freshness check for agent-push model
    if (host.lastAgentUpdate) {
      const age = Date.now() - new Date(host.lastAgentUpdate).getTime();
      if (age > 30000) {
        // Data is stale (>30s), mark as offline
        host.state = 'offline';
      }
    }

    return host;
  }

  /**
   * Get all hosts with freshness check for agent-push data
   */
  getAllHosts() {
    return Array.from(this.hosts.values()).map(h => this.getHost(h.id));
  }

  /**
   * Get hosts by state
   */
  getHostsByState(state) {
    return this.getAllHosts().filter(h => h.state === state);
  }

  /**
   * Get host summary with freshness check
   */
  getSummary() {
    const hosts = this.getAllHosts();
    const byState = { online: 0, offline: 0, unknown: 0 };

    for (const host of hosts) {
      // Apply freshness check via getHost
      const freshHost = this.getHost(host.id);
      byState[freshHost.state] = (byState[freshHost.state] || 0) + 1;
    }

    return {
      total: hosts.length,
      byState,
      poolSize: this.connectionPool.size
    };
  }

  // ============ Cleanup ============

  /**
   * Close all connections and stop all heartbeats
   */
  shutdown() {
    // Close all connections
    for (const [hostId] of this.connectionPool) {
      this.closeConnection(hostId);
    }

    // Stop all heartbeats
    for (const [hostId] of this.heartbeatTimers) {
      this.stopHeartbeat(hostId);
    }

    this.emit('shutdown');
  }
}

module.exports = PhysicalHostManager;