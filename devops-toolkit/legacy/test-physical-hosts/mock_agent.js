#!/usr/bin/env node
/**
 * Mock Telegraf Agent
 * 模拟 Telegraf agent 行为:
 * 1. 收集指标 (模拟)
 * 2. 写入 InfluxDB
 * 3. 暴露 /metrics 给 Prometheus 拉取
 */

const http = require('http');
const os = require('os');

// Configuration from env
const INFLUXDB_URL = process.env.INFLUXDB_URL || 'http://localhost:8086';
const INFLUXDB_TOKEN = process.env.INFLUXDB_TOKEN || 'devops-token';
const INFLUXDB_ORG = process.env.INFLUXDB_ORG || 'devops';
const INFLUXDB_BUCKET = process.env.INFLUXDB_BUCKET || 'telegraf';
const METRICS_INTERVAL = parseInt(process.env.METRICS_INTERVAL || '10000');
const TARGET_HOSTS = (process.env.TARGET_HOSTS || 'host-1,host-2,host-3').split(',');

let metricsCache = {};

// ============================================================
// InfluxDB Write
// ============================================================

async function writeToInfluxDB(metrics) {
  const timestamp = Date.now() * 1000000; // nanoseconds

  const lines = [];

  // CPU metrics
  if (metrics.cpu) {
    lines.push(`cpu,host=${metrics.hostname},host_id=${metrics.hostId} usage=${metrics.cpu.usage || 0},cores=${metrics.cpu.cores || 1},user=${metrics.cpu.user || 0},system=${metrics.cpu.system || 0},idle=${metrics.cpu.idle || 100} ${timestamp}`);
  }

  // Memory metrics
  if (metrics.memory) {
    lines.push(`mem,host=${metrics.hostname},host_id=${metrics.hostId} total=${metrics.memory.total || 0},free=${metrics.memory.free || 0},used=${metrics.memory.used || 0},usage_percent=${metrics.memory.usagePercent || 0} ${timestamp}`);
  }

  // Uptime
  if (metrics.uptime) {
    lines.push(`uptime,host=${metrics.hostname},host_id=${metrics.hostId} value=${metrics.uptime.value || 0} ${timestamp}`);
  }

  if (lines.length === 0) return;

  const body = lines.join('\n');

  try {
    const url = new URL('/api/v2/write', INFLUXDB_URL);
    const response = await fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Authorization': `Token ${INFLUXDB_TOKEN}`,
        'Content-Type': 'text/plain',
        'X-Organization': INFLUXDB_ORG,
        'X-Bucket-Name': INFLUXDB_BUCKET
      },
      body
    });

    if (!response.ok) {
      console.log(`[InfluxDB] Write failed: ${response.status} ${response.statusText}`);
    } else {
      console.log(`[InfluxDB] Wrote ${lines.length} metrics for ${metrics.hostname}`);
    }
  } catch (err) {
    console.log(`[InfluxDB] Write error: ${err.message}`);
  }
}

// ============================================================
// Mock Metrics Collection (模拟从 SSH 连接收集的数据)
// ============================================================

function collectMockMetrics(hostId, hostname) {
  // 生成模拟数据
  const cpuUsage = Math.random() * 80 + 10; // 10-90%
  const memTotal = 8192; // 8GB
  const memUsed = Math.floor(memTotal * (0.3 + Math.random() * 0.4)); // 30-70%
  const uptimeSeconds = Math.floor(Math.random() * 8640000); // 0-100 days

  const metrics = {
    hostId,
    hostname,
    cpu: {
      usage: parseFloat(cpuUsage.toFixed(2)),
      cores: os.cpus().length || 4,
      user: Math.floor(cpuUsage * 100),
      system: Math.floor(cpuUsage * 50),
      idle: Math.floor(100 - cpuUsage)
    },
    memory: {
      total: memTotal,
      free: memTotal - memUsed,
      used: memUsed,
      usagePercent: parseFloat(((memUsed / memTotal) * 100).toFixed(2))
    },
    disk: {
      disks: [
        { device: '/dev/sda', size: '100G', used: '50G', percent: 50 }
      ]
    },
    uptime: {
      value: uptimeSeconds,
      formatted: formatUptime(uptimeSeconds)
    },
    services: {
      ssh: { status: 'running', uptime: uptimeSeconds },
      cron: { status: 'running', uptime: uptimeSeconds }
    },
    collectedAt: new Date().toISOString()
  };

  return metrics;
}

function formatUptime(seconds) {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return `${days}d ${hours}h ${mins}m`;
}

// ============================================================
// Prometheus /metrics Endpoint
// ============================================================

function generatePrometheusMetrics() {
  const lines = [];
  const timestamp = Date.now() / 1000; // seconds

  for (const [hostId, metrics] of Object.entries(metricsCache)) {
    const labels = `host_id="${hostId}",hostname="${metrics.hostname}"`;

    // CPU
    if (metrics.cpu) {
      lines.push(`# TYPE cpu_usage gauge`);
      lines.push(`cpu_usage{${labels}} ${metrics.cpu.usage || 0}`);
    }

    // Memory
    if (metrics.memory) {
      lines.push(`# TYPE memory_usage_bytes gauge`);
      lines.push(`memory_usage_bytes{${labels}} ${(metrics.memory.used || 0) * 1024 * 1024}`);
      lines.push(`# TYPE memory_total_bytes gauge`);
      lines.push(`memory_total_bytes{${labels}} ${(metrics.memory.total || 0) * 1024 * 1024}`);
    }

    // Uptime
    if (metrics.uptime) {
      lines.push(`# TYPE uptime_seconds gauge`);
      lines.push(`uptime_seconds{${labels}} ${metrics.uptime.value || 0}`);
    }

    // State
    lines.push(`# TYPE host_state gauge`);
    lines.push(`host_state{${labels}} 1`);

    // Last update
    lines.push(`# TYPE host_last_update_timestamp gauge`);
    lines.push(`host_last_update_timestamp{${labels}} ${timestamp}`);
  }

  // Info metric
  lines.push(`# TYPE devops_mock_agent_info gauge`);
  lines.push(`devops_mock_agent_info{service="mock-telegraf-agent"} 1`);

  return lines.join('\n') + '\n';
}

// ============================================================
// HTTP Server
// ============================================================

const server = http.createServer((req, res) => {
  const parsedUrl = new URL(req.url, `http://${req.headers.host}`);

  // /metrics - Prometheus scraping endpoint
  if (parsedUrl.pathname === '/metrics' && req.method === 'GET') {
    const metrics = generatePrometheusMetrics();
    res.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' });
    res.end(metrics);
    return;
  }

  // /health - Health check
  if (parsedUrl.pathname === '/health' && req.method === 'GET') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', timestamp: new Date().toISOString() }));
    return;
  }

  // /metrics (JSON) - Direct JSON metrics
  if (parsedUrl.pathname === '/metrics.json' && req.method === 'GET') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(metricsCache, null, 2));
    return;
  }

  // 404
  res.writeHead(404);
  res.end('Not Found');
});

// ============================================================
// Main Loop
// ============================================================

async function main() {
  console.log('==========================================');
  console.log('   Mock Telegraf Agent Started');
  console.log('==========================================');
  console.log(`InfluxDB: ${INFLUXDB_URL}`);
  console.log(`Org/Bucket: ${INFLUXDB_ORG}/${INFLUXDB_BUCKET}`);
  console.log(`Interval: ${METRICS_INTERVAL}ms`);
  console.log(`Target Hosts: ${TARGET_HOSTS.join(', ')}`);
  console.log('');

  // Start HTTP server
  const PORT = 8090;
  server.listen(PORT, () => {
    console.log(`Mock Agent HTTP server running on port ${PORT}`);
    console.log(`  - GET /metrics (Prometheus format)`);
    console.log(`  - GET /metrics.json (JSON format)`);
    console.log(`  - GET /health`);
    console.log('');
  });

  // Collect and write metrics
  async function collectAndWrite() {
    console.log(`[${new Date().toISOString()}] Collecting metrics...`);

    for (const host of TARGET_HOSTS) {
      const [hostname, portStr] = host.split(':');
      const hostId = hostname.replace('host-', 'physical-host-');

      const metrics = collectMockMetrics(hostId, hostname);
      metricsCache[hostId] = metrics;

      // Write to InfluxDB
      await writeToInfluxDB(metrics);
    }

    console.log(`[${new Date().toISOString()}] Collection complete`);
  }

  // Initial collection
  await collectAndWrite();

  // Periodic collection
  setInterval(collectAndWrite, METRICS_INTERVAL);
}

main().catch(console.error);
