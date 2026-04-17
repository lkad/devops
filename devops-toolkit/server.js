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

// Initialize device manager
const deviceManager = new DeviceManager(path.join(__dirname, 'config/devices'));

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

    // GET /api/devices/:id/metrics - Get device metrics
    if (method === 'GET' && pathname.match(/^\/api\/devices\/[^/]+\/metrics$/)) {
      const id = pathname.split('/')[3];
      const device = deviceManager.getDevice(id);
      if (device) {
        // 返回模拟的监控数据
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
        // 返回模拟的事件日志
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

    // POST /api/devices/:id/actions - Execute device action
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

        // 模拟操作执行
        const actions = ['restart', 'stop', 'pause', 'resume'];
        if (!actions.includes(action)) {
          sendJSON(res, 400, { success: false, error: 'Invalid action' });
          return;
        }

        // 返回操作结果
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

  // 404 for unknown routes
  res.writeHead(404);
  res.end('Not found');
}

// Create and start server
const PORT = process.env.PORT || 3000;
const server = http.createServer(handleRequest);

server.listen(PORT, () => {
  console.log(`\n╔═══════════════════════════════════════════╗`);
  console.log(`║   DevOps Toolkit Web Server              ║`);
  console.log(`╚═══════════════════════════════════════════╝\n`);
  console.log(`🌐 Server running at http://localhost:${PORT}`);
  console.log(`📊 Device API: http://localhost:${PORT}/api/devices`);
  console.log(`❤️  Health: http://localhost:${PORT}/health\n`);
});

module.exports = server;
