/**
 * WebSocket Manager
 * Handles real-time connections for log streaming, metrics, and events
 */

const { WebSocketServer } = require('ws');

class WebSocketManager {
  constructor() {
    this.clients = new Map(); // clientId -> { ws, subscriptions }
    this.nextClientId = 1;

    // Event types
    this.EVENT_TYPES = {
      LOG: 'log',
      METRIC: 'metric',
      DEVICE_EVENT: 'device_event',
      PIPELINE_UPDATE: 'pipeline_update',
      ALERT: 'alert'
    };
  }

  initialize(server) {
    this.wss = new WebSocketServer({ server, path: '/ws' });

    this.wss.on('connection', (ws, req) => {
      const clientId = this.nextClientId++;
      this.clients.set(clientId, { ws, subscriptions: new Set() });

      console.log(`[WS] Client ${clientId} connected`);

      // Send welcome message
      this.sendToClient(clientId, {
        type: 'connected',
        clientId,
        timestamp: new Date().toISOString()
      });

      ws.on('message', (message) => {
        try {
          const data = JSON.parse(message);
          this.handleMessage(clientId, data);
        } catch (e) {
          console.error(`[WS] Invalid message from client ${clientId}:`, e.message);
        }
      });

      ws.on('close', () => {
        console.log(`[WS] Client ${clientId} disconnected`);
        this.clients.delete(clientId);
      });

      ws.on('error', (err) => {
        console.error(`[WS] Client ${clientId} error:`, err.message);
        this.clients.delete(clientId);
      });
    });

    console.log('[WS] WebSocket server initialized on /ws');
  }

  handleMessage(clientId, data) {
    const client = this.clients.get(clientId);
    if (!client) return;

    switch (data.action) {
      case 'subscribe':
        if (data.channel) {
          client.subscriptions.add(data.channel);
          this.sendToClient(clientId, {
            type: 'subscribed',
            channel: data.channel
          });
          console.log(`[WS] Client ${clientId} subscribed to ${data.channel}`);
        }
        break;

      case 'unsubscribe':
        if (data.channel) {
          client.subscriptions.delete(data.channel);
          this.sendToClient(clientId, {
            type: 'unsubscribed',
            channel: data.channel
          });
        }
        break;

      case 'ping':
        this.sendToClient(clientId, { type: 'pong', timestamp: Date.now() });
        break;
    }
  }

  sendToClient(clientId, data) {
    const client = this.clients.get(clientId);
    if (client && client.ws.readyState === 1) { // OPEN
      client.ws.send(JSON.stringify(data));
    }
  }

  broadcast(channel, data) {
    const message = JSON.stringify({
      channel,
      ...data,
      timestamp: new Date().toISOString()
    });

    for (const [clientId, client] of this.clients) {
      if (client.subscriptions.has(channel) && client.ws.readyState === 1) {
        client.ws.send(message);
      }
    }
  }

  // Broadcast log event to subscribed clients
  broadcastLog(log) {
    this.broadcast(this.EVENT_TYPES.LOG, {
      type: this.EVENT_TYPES.LOG,
      data: log
    });
  }

  // Broadcast metric update
  broadcastMetric(metric) {
    this.broadcast(this.EVENT_TYPES.METRIC, {
      type: this.EVENT_TYPES.METRIC,
      data: metric
    });
  }

  // Broadcast device event
  broadcastDeviceEvent(event) {
    this.broadcast(this.EVENT_TYPES.DEVICE_EVENT, {
      type: this.EVENT_TYPES.DEVICE_EVENT,
      data: event
    });
  }

  // Broadcast pipeline update
  broadcastPipelineUpdate(update) {
    this.broadcast(this.EVENT_TYPES.PIPELINE_UPDATE, {
      type: this.EVENT_TYPES.PIPELINE_UPDATE,
      data: update
    });
  }

  // Broadcast alert
  broadcastAlert(alert) {
    this.broadcast(this.EVENT_TYPES.ALERT, {
      type: this.EVENT_TYPES.ALERT,
      data: alert
    });
  }

  getClientCount() {
    return this.clients.size;
  }

  getChannelStats() {
    const stats = {};
    for (const [clientId, client] of this.clients) {
      for (const channel of client.subscriptions) {
        stats[channel] = (stats[channel] || 0) + 1;
      }
    }
    return stats;
  }
}

// Singleton instance
const wsManager = new WebSocketManager();

module.exports = wsManager;