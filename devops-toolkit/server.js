#!/usr/bin/env node

/**
 * DevOps Toolkit - Web Server
 * Serves frontend and provides REST API for device management
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const url = require('url');

// DeviceManager for actual device operations
const DeviceManager = require('./devices/device_manager');
const PipelineManager = require('./pipelines/pipeline_manager');
const LogManager = require('./logs/log_manager');
const wsManager = require('./websocket_manager');
const MetricsManager = require('./metrics_manager');
const AlertNotificationManager = require('./alerts_notification_manager');

// Initialize managers
const deviceManager = new DeviceManager(path.join(__dirname, 'config/devices'));
const pipelineManager = new PipelineManager(path.join(__dirname, 'config/pipelines.json'));
const logManager = new LogManager(path.join(__dirname, 'config/logs.json'));
const metricsManager = require('./metrics_manager');
const alertNotificationManager = require('./alerts_notification_manager');

// Wire up LogManager callbacks
logManager.setCallbacks(
  // onLogAdded - broadcast to WebSocket
  (log) => {
    wsManager.broadcastLog(log);
    metricsManager.recordLog(log.level);
  },
  // onAlertTriggered - send notifications
  (alert) => {
    alertNotificationManager.triggerAlert(alert);
    metricsManager.recordAlert(alert.name, alert.severity);
  }
);

// Configure alert channels from environment
alertNotificationManager.configureFromEnv();

// MIME types
const MIME_TYPES = {
  '.html': 'text/html',
  '.css': 'text/css',
  '.js': 'application/javascript',
  '.json': 'application/json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon'
};

// Parse JSON body
function parseBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try { resolve(body ? JSON.parse(body) : {}); }
      catch (e) { reject(new Error('Invalid JSON')); }
    });
    req.on('error', reject);
  });
}

// Send JSON response
function sendJSON(res, status, data) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(data));
}

// Send HTML response
function sendHTML(res, html) {
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end(html);
}

// Route handler
async function handleRequest(req, res) {
  const startTime = Date.now();
  const parsedUrl = url.parse(req.url, true);
  const pathname = parsedUrl.pathname;
  const method = req.method;

  // CORS headers
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }

  // Wrap sendJSON to record metrics
  const originalSendJSON = sendJSON;
  sendJSON = function(res, status, data) {
    const duration = Date.now() - startTime;
    // Record HTTP metrics for API requests
    if (pathname.startsWith('/api/')) {
      metricsManager.recordLatency(pathname, method, status, duration);
    }
    originalSendJSON(res, status, data);
    sendJSON = originalSendJSON;
  };

  // API routes
  if (pathname.startsWith('/api/')) {
    // GET /api/devices - List all devices
    if (method === 'GET' && pathname === '/api/devices') {
      const devices = deviceManager.getAllDevices();
      sendJSON(res, 200, { success: true, devices });
      return;
    }

    // GET /api/devices/search?tags=env:dev,type:web - Search by tags (must be before /:id route)
    if (method === 'GET' && pathname === '/api/devices/search') {
      const tags = parsedUrl.query.tags ? parsedUrl.query.tags.split(',') : [];
      const devices = deviceManager.getDevicesByTags(tags);
      sendJSON(res, 200, { success: true, devices });
      return;
    }

    // GET /api/devices/:id/metrics - Get device metrics
    if (method === 'GET' && pathname.match(/^\/api\/devices\/[^/]+\/metrics$/)) {
      const id = pathname.split('/')[3];
      const device = deviceManager.getDevice(id);
      if (device) {
        const metrics = {
          device_id: id,
          timestamp: new Date().toISOString(),
          cpu: { usage: Math.random() * 100, cores: 4 },
          memory: { used: Math.floor(Math.random() * 16), total: 16, unit: 'GB' },
          network: { rx: Math.floor(Math.random() * 1000), tx: Math.floor(Math.random() * 1000), unit: 'MB/s' },
          disk: { used: Math.floor(Math.random() * 500), total: 1000, unit: 'GB' }
        };
        sendJSON(res, 200, { success: true, metrics });
      } else {
        sendJSON(res, 404, { success: false, error: 'Device not found' });
      }
      return;
    }

    // GET /api/devices/:id/events - Get device events
    if (method === 'GET' && pathname.match(/^\/api\/devices\/[^/]+\/events$/)) {
      const id = pathname.split('/')[3];
      const device = deviceManager.getDevice(id);
      if (device) {
        const events = [
          { id: 1, type: 'status_change', message: '设备上线', timestamp: new Date(Date.now() - 3600000).toISOString() },
          { id: 2, type: 'config_update', message: '配置已更新', timestamp: new Date(Date.now() - 7200000).toISOString() },
          { id: 3, type: 'heartbeat', message: '心跳检测成功', timestamp: new Date(Date.now() - 10800000).toISOString() },
        ];
        sendJSON(res, 200, { success: true, events });
      } else {
        sendJSON(res, 404, { success: false, error: 'Device not found' });
      }
      return;
    }

    // GET /api/devices/:id/actions - Execute device action
    if (method === 'POST' && pathname.match(/^\/api\/devices\/[^/]+\/actions$/)) {
      const id = pathname.split('/')[3];
      const device = deviceManager.getDevice(id);
      if (!device) {
        sendJSON(res, 404, { success: false, error: 'Device not found' });
        return;
      }
      try {
        const body = await parseBody(req);
        const { action } = body;
        const actions = ['restart', 'stop', 'pause', 'resume'];
        if (!actions.includes(action)) {
          sendJSON(res, 400, { success: false, error: 'Invalid action' });
          return;
        }
        const result = {
          success: true,
          action,
          device_id: id,
          status: 'executed',
          executed_at: new Date().toISOString()
        };
        sendJSON(res, 200, result);
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // GET /api/devices/:id - Get single device
    if (method === 'GET' && pathname.startsWith('/api/devices/')) {
      const id = pathname.split('/')[3];
      const device = deviceManager.getDevice(id);
      if (device) {
        sendJSON(res, 200, { success: true, device });
      } else {
        sendJSON(res, 404, { success: false, error: 'Device not found' });
      }
      return;
    }

    // POST /api/devices - Create device
    if (method === 'POST' && pathname === '/api/devices') {
      try {
        const body = await parseBody(req);
        const device = deviceManager.registerDevice({
          id: body.id,
          type: body.type,
          name: body.name,
          labels: body.labels || [],
          business_unit: body.business_unit || null,
          compute_cluster: body.compute_cluster || null
        });
        metricsManager.recordDeviceEvent('registered');
        wsManager.broadcastDeviceEvent({ type: 'registered', device });
        sendJSON(res, 201, { success: true, device });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // PUT /api/devices/:id - Update device
    if (method === 'PUT' && pathname.startsWith('/api/devices/')) {
      const id = pathname.split('/')[3];
      try {
        const body = await parseBody(req);
        const result = deviceManager.updateConfig(id, body);
        if (result.success) {
          sendJSON(res, 200, result);
        } else {
          sendJSON(res, 404, result);
        }
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // DELETE /api/devices/:id - Delete device
    if (method === 'DELETE' && pathname.startsWith('/api/devices/')) {
      const id = pathname.split('/')[3];
      const result = deviceManager.removeDevice(id);
      if (result.success) {
        sendJSON(res, 200, result);
      } else {
        sendJSON(res, 404, result);
      }
      return;
    }

    // ============ Pipeline API Routes ============

    // GET /api/pipelines - List all pipelines
    if (method === 'GET' && pathname === '/api/pipelines') {
      const pipelines = pipelineManager.getAllPipelines();
      sendJSON(res, 200, { success: true, pipelines });
      return;
    }

    // POST /api/pipelines - Create pipeline
    if (method === 'POST' && pathname === '/api/pipelines') {
      try {
        const body = await parseBody(req);
        const pipeline = pipelineManager.createPipeline(body);
        sendJSON(res, 201, { success: true, pipeline });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // GET /api/pipelines/:id - Get pipeline
    if (method === 'GET' && pathname.match(/^\/api\/pipelines\/[^/]+$/)) {
      const id = pathname.split('/')[3];
      const pipeline = pipelineManager.getPipeline(id);
      if (pipeline) {
        const stats = pipelineManager.getPipelineStats(id);
        sendJSON(res, 200, { success: true, pipeline, stats });
      } else {
        sendJSON(res, 404, { success: false, error: 'Pipeline not found' });
      }
      return;
    }

    // PUT /api/pipelines/:id - Update pipeline
    if (method === 'PUT' && pathname.match(/^\/api\/pipelines\/[^/]+$/)) {
      const id = pathname.split('/')[3];
      try {
        const body = await parseBody(req);
        const pipeline = pipelineManager.updatePipeline(id, body);
        if (pipeline) {
          sendJSON(res, 200, { success: true, pipeline });
        } else {
          sendJSON(res, 404, { success: false, error: 'Pipeline not found' });
        }
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // DELETE /api/pipelines/:id - Delete pipeline
    if (method === 'DELETE' && pathname.match(/^\/api\/pipelines\/[^/]+$/)) {
      const id = pathname.split('/')[3];
      const result = pipelineManager.deletePipeline(id);
      if (result) {
        sendJSON(res, 200, { success: true });
      } else {
        sendJSON(res, 404, { success: false, error: 'Pipeline not found' });
      }
      return;
    }

    // POST /api/pipelines/:id/execute - Execute pipeline
    if (method === 'POST' && pathname.match(/^\/api\/pipelines\/[^/]+\/execute$/)) {
      const id = pathname.split('/')[3];
      try {
        const body = await parseBody(req);
        const run = pipelineManager.executePipeline(id, {
          type: 'manual',
          triggered_by: body.triggered_by || 'api',
          ...body
        });
        if (run) {
          metricsManager.recordPipelineEvent('executed', id);
          wsManager.broadcastPipelineUpdate({ pipeline_id: id, run, action: 'executed' });
          sendJSON(res, 201, { success: true, run });
        } else {
          sendJSON(res, 404, { success: false, error: 'Pipeline not found' });
        }
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // GET /api/pipelines/:id/runs - Get pipeline runs
    if (method === 'GET' && pathname.match(/^\/api\/pipelines\/[^/]+\/runs$/)) {
      const id = pathname.split('/')[3];
      const runs = pipelineManager.getPipelineRuns(id);
      sendJSON(res, 200, { success: true, runs });
      return;
    }

    // GET /api/pipelines/:id/runs/:runId - Get run details
    if (method === 'GET' && pathname.match(/^\/api\/pipelines\/[^/]+\/runs\/[^/]+$/)) {
      const parts = pathname.split('/');
      const runId = parts[parts.length - 1];
      const run = pipelineManager.getRun(runId);
      if (run) {
        sendJSON(res, 200, { success: true, run });
      } else {
        sendJSON(res, 404, { success: false, error: 'Run not found' });
      }
      return;
    }

    // POST /api/pipelines/:id/runs/:runId/cancel - Cancel running pipeline
    if (method === 'POST' && pathname.match(/^\/api\/pipelines\/[^/]+\/runs\/[^/]+\/cancel$/)) {
      const parts = pathname.split('/');
      const runId = parts[parts.length - 2];
      const run = pipelineManager.cancelRun(runId);
      if (run) {
        sendJSON(res, 200, { success: true, run });
      } else {
        sendJSON(res, 404, { success: false, error: 'Run not found' });
      }
      return;
    }

    // GET /api/runs - Get all recent runs
    if (method === 'GET' && pathname === '/api/runs') {
      const runs = pipelineManager.getRecentRuns(50);
      sendJSON(res, 200, { success: true, runs });
      return;
    }

    // ============ Log API Routes ============

    // GET /api/logs - Query logs
    if (method === 'GET' && pathname === '/api/logs') {
      const query = parsedUrl.query;
      const options = {
        level: query.level || null,
        levels: query.levels ? query.levels.split(',') : null,
        source: query.source || null,
        resource: query.resource || null,
        search: query.search || null,
        start_time: query.start_time || null,
        end_time: query.end_time || null,
        tags: query.tags ? query.tags.split(',') : null,
        offset: parseInt(query.offset) || 0,
        limit: parseInt(query.limit) || 100,
        order: query.order || 'desc'
      };

      // For ES/Loki backends, use async query; for local, use sync
      const isESLoki = logManager.backend.constructor.name !== 'LocalStorageBackend';
      let result;
      if (isESLoki) {
        result = await logManager.queryLogsBackend(options);
      } else {
        result = logManager.queryLogs(options);
      }
      sendJSON(res, 200, { success: true, ...result });
      return;
    }

    // GET /api/logs/stats - Get log statistics
    if (method === 'GET' && pathname === '/api/logs/stats') {
      const stats = logManager.getLogStats();
      sendJSON(res, 200, { success: true, stats });
      return;
    }

    // POST /api/logs - Add log entry
    if (method === 'POST' && pathname === '/api/logs') {
      try {
        const body = await parseBody(req);
        const log = logManager.addLog(body);
        sendJSON(res, 201, { success: true, log });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // POST /api/logs/generate - Generate sample logs (for demo)
    if (method === 'POST' && pathname === '/api/logs/generate') {
      const body = await parseBody(req).catch(() => ({}));
      const count = body.count || 50;
      const result = logManager.generateSampleLogs(count);
      sendJSON(res, 200, { success: true, ...result });
      return;
    }

    // GET /api/logs/alerts - Get alert rules
    if (method === 'GET' && pathname === '/api/logs/alerts') {
      const alerts = logManager.getAlertRules();
      sendJSON(res, 200, { success: true, alerts });
      return;
    }

    // POST /api/logs/alerts - Create alert rule
    if (method === 'POST' && pathname === '/api/logs/alerts') {
      try {
        const body = await parseBody(req);
        const alert = logManager.createAlertRule(body);
        sendJSON(res, 201, { success: true, alert });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // PUT /api/logs/alerts/:id - Update alert rule
    if (method === 'PUT' && pathname.match(/^\/api\/logs\/alerts\/[^/]+$/)) {
      const id = pathname.split('/')[4];
      try {
        const body = await parseBody(req);
        const alert = logManager.updateAlertRule(id, body);
        if (alert) {
          sendJSON(res, 200, { success: true, alert });
        } else {
          sendJSON(res, 404, { success: false, error: 'Alert not found' });
        }
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // DELETE /api/logs/alerts/:id - Delete alert rule
    if (method === 'DELETE' && pathname.match(/^\/api\/logs\/alerts\/[^/]+$/)) {
      const id = pathname.split('/')[4];
      const result = logManager.deleteAlertRule(id);
      if (result) {
        sendJSON(res, 200, { success: true });
      } else {
        sendJSON(res, 404, { success: false, error: 'Alert not found' });
      }
      return;
    }

    // GET /api/logs/filters - Get saved filters
    if (method === 'GET' && pathname === '/api/logs/filters') {
      const filters = logManager.getSavedFilters();
      sendJSON(res, 200, { success: true, filters });
      return;
    }

    // POST /api/logs/filters - Save filter
    if (method === 'POST' && pathname === '/api/logs/filters') {
      try {
        const body = await parseBody(req);
        const filter = logManager.saveFilter(body);
        sendJSON(res, 201, { success: true, filter });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // GET /api/logs/retention - Get retention configuration
    if (method === 'GET' && pathname === '/api/logs/retention') {
      const config = logManager.getRetentionConfig();
      sendJSON(res, 200, { success: true, config });
      return;
    }

    // PUT /api/logs/retention - Update retention configuration
    if (method === 'PUT' && pathname === '/api/logs/retention') {
      try {
        const body = await parseBody(req);
        const config = logManager.updateRetentionConfig(body);
        sendJSON(res, 200, { success: true, config });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // POST /api/logs/retention/apply - Apply retention policy
    if (method === 'POST' && pathname === '/api/logs/retention/apply') {
      try {
        const result = await logManager.applyRetention();
        sendJSON(res, 200, { success: true, ...result });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // GET /api/logs/backend - Get storage backend health
    if (method === 'GET' && pathname === '/api/logs/backend') {
      const health = await logManager.getBackendHealth();
      sendJSON(res, 200, { success: true, health });
      return;
    }

    // ============ Metrics API Routes ============

    // GET /api/metrics - Get all metrics as JSON
    if (method === 'GET' && pathname === '/api/metrics') {
      const data = metricsManager.getMetricsJSON();
      sendJSON(res, 200, { success: true, ...data });
      return;
    }

    // POST /api/metrics/counter - Increment a counter
    if (method === 'POST' && pathname === '/api/metrics/counter') {
      try {
        const body = await parseBody(req);
        metricsManager.incCounter(body.name, body.labels || {}, body.value || 1);
        sendJSON(res, 200, { success: true });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // POST /api/metrics/gauge - Set/inc/dec a gauge
    if (method === 'POST' && pathname === '/api/metrics/gauge') {
      try {
        const body = await parseBody(req);
        if (body.action === 'inc') {
          metricsManager.incGauge(body.name, body.labels || {}, body.value || 1);
        } else if (body.action === 'dec') {
          metricsManager.decGauge(body.name, body.labels || {}, body.value || 1);
        } else {
          metricsManager.setGauge(body.name, body.labels || {}, body.value);
        }
        sendJSON(res, 200, { success: true });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // POST /api/metrics/histogram - Observe a histogram value
    if (method === 'POST' && pathname === '/api/metrics/histogram') {
      try {
        const body = await parseBody(req);
        metricsManager.observeHistogram(body.name, body.labels || {}, body.value);
        sendJSON(res, 200, { success: true });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // ============ Alert Notification API Routes ============

    // GET /api/alerts/channels - List notification channels
    if (method === 'GET' && pathname === '/api/alerts/channels') {
      const channels = alertNotificationManager.getChannels();
      sendJSON(res, 200, { success: true, channels });
      return;
    }

    // POST /api/alerts/channels - Add notification channel
    if (method === 'POST' && pathname === '/api/alerts/channels') {
      try {
        const body = await parseBody(req);
        alertNotificationManager.addChannel(body.name, body.config);
        sendJSON(res, 201, { success: true });
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // DELETE /api/alerts/channels/:name - Remove channel
    if (method === 'DELETE' && pathname.match(/^\/api\/alerts\/channels\/[^/]+$/)) {
      const name = pathname.split('/')[4];
      alertNotificationManager.removeChannel(name);
      sendJSON(res, 200, { success: true });
      return;
    }

    // GET /api/alerts/history - Get alert history
    if (method === 'GET' && pathname === '/api/alerts/history') {
      const query = parsedUrl.query;
      const options = {};
      if (query.severity) options.severity = query.severity;
      if (query.since) options.since = query.since;
      if (query.limit) options.limit = parseInt(query.limit);
      const history = alertNotificationManager.getHistory(options);
      sendJSON(res, 200, { success: true, history });
      return;
    }

    // GET /api/alerts/stats - Get alert statistics
    if (method === 'GET' && pathname === '/api/alerts/stats') {
      const stats = alertNotificationManager.getStats();
      sendJSON(res, 200, { success: true, stats });
      return;
    }

    // POST /api/alerts/trigger - Trigger an alert
    if (method === 'POST' && pathname === '/api/alerts/trigger') {
      try {
        const body = await parseBody(req);
        const result = await alertNotificationManager.triggerAlert(body);
        wsManager.broadcastAlert(body);
        sendJSON(res, 200, result);
      } catch (error) {
        sendJSON(res, 400, { success: false, error: error.message });
      }
      return;
    }

    // ============ End Log API Routes ============

    // Unknown API route
    sendJSON(res, 404, { success: false, error: 'API endpoint not found' });
    return;
  }

  // Serve static files from frontend directory
  if (pathname === '/' || pathname === '/index.html') {
    const htmlPath = path.join(__dirname, 'frontend', 'index.html');
    fs.readFile(htmlPath, (err, data) => {
      if (err) {
        res.writeHead(404);
        res.end('Not found');
        return;
      }
      sendHTML(res, data.toString());
    });
    return;
  }

  // Health check
  if (pathname === '/health') {
    sendJSON(res, 200, { status: 'healthy', timestamp: new Date().toISOString() });
    return;
  }

  // Prometheus metrics endpoint
  if (pathname === '/metrics') {
    const metrics = metricsManager.exportPrometheus();
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end(metrics);
    return;
  }

  // 404 for unknown routes
  res.writeHead(404);
  res.end('Not found');
}

// Create and start server
const PORT = process.env.PORT || 3000;
const server = http.createServer(handleRequest);

// Initialize WebSocket
wsManager.initialize(server);

server.listen(PORT, () => {
  console.log(`\n╔═══════════════════════════════════════════╗`);
  console.log(`║   DevOps Toolkit Web Server              ║`);
  console.log(`╚═══════════════════════════════════════════╝\n`);
  console.log(`🌐 Server running at http://localhost:${PORT}`);
  console.log(`📊 Device API: http://localhost:${PORT}/api/devices`);
  console.log(`📈 Metrics: http://localhost:${PORT}/metrics`);
  console.log(`🔌 WebSocket: ws://localhost:${PORT}/ws`);
  console.log(`❤️  Health: http://localhost:${PORT}/health\n`);
});

module.exports = server;
