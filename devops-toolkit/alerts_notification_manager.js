/**
 * Alert Notification Manager
 * Handles routing alerts to different notification channels
 */

const https = require('https');
const http = require('http');
const { URL } = require('url');

class AlertNotificationManager {
  constructor() {
    // Configured notification channels
    this.channels = new Map();

    // Alert history
    this.alertHistory = [];
    this.maxHistorySize = 1000;

    // Rate limiting
    this.rateLimits = new Map();
    this.windowMs = 60000; // 1 minute window
    this.maxAlertsPerWindow = 10;
  }

  // Configure a notification channel
  addChannel(name, config) {
    this.channels.set(name, {
      ...config,
      enabled: config.enabled !== false
    });
    console.log(`[AlertManager] Added channel: ${name}`);
  }

  // Remove a channel
  removeChannel(name) {
    this.channels.delete(name);
  }

  // Get all channels
  getChannels() {
    return Array.from(this.channels.entries()).map(([name, config]) => ({
      name,
      ...config
    }));
  }

  // Check rate limit
  checkRateLimit(alertName) {
    const key = `rate:${alertName}`;
    const now = Date.now();

    if (!this.rateLimits.has(key)) {
      this.rateLimits.set(key, { count: 1, windowStart: now });
      return true;
    }

    const limit = this.rateLimits.get(key);
    if (now - limit.windowStart > this.windowMs) {
      // Reset window
      limit.count = 1;
      limit.windowStart = now;
      return true;
    }

    if (limit.count >= this.maxAlertsPerWindow) {
      return false;
    }

    limit.count++;
    return true;
  }

  // Send alert to a specific channel
  async sendToChannel(channel, alert) {
    if (!channel.enabled) {
      return { success: false, reason: 'Channel disabled' };
    }

    try {
      switch (channel.type) {
        case 'webhook':
          return await this.sendWebhook(channel, alert);
        case 'slack':
          return await this.sendSlack(channel, alert);
        case 'email':
          return await this.sendEmail(channel, alert);
        case 'log':
          return this.sendToLog(channel, alert);
        default:
          return { success: false, reason: `Unknown channel type: ${channel.type}` };
      }
    } catch (e) {
      return { success: false, reason: e.message };
    }
  }

  // Send via webhook
  async sendWebhook(channel, alert) {
    const url = new URL(channel.url);
    const protocol = url.protocol === 'https:' ? https : http;

    return new Promise((resolve) => {
      const data = JSON.stringify({
        alert: alert.name,
        message: alert.message,
        severity: alert.severity,
        source: alert.source,
        timestamp: alert.timestamp,
        metadata: alert.metadata
      });

      const options = {
        hostname: url.hostname,
        port: url.port || (url.protocol === 'https:' ? 443 : 80),
        path: url.pathname,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(data)
        }
      };

      if (channel.headers) {
        Object.assign(options.headers, channel.headers);
      }

      const req = protocol.request(options, (res) => {
        let body = '';
        res.on('data', chunk => body += chunk);
        res.on('end', () => {
          resolve({
            success: res.statusCode >= 200 && res.statusCode < 300,
            statusCode: res.statusCode,
            response: body.substring(0, 200)
          });
        });
      });

      req.on('error', (e) => {
        resolve({ success: false, reason: e.message });
      });

      req.write(data);
      req.end();
    });
  }

  // Send to Slack
  async sendSlack(channel, alert) {
    const emoji = {
      critical: ':rotating_light:',
      high: ':warning:',
      medium: ':large_yellow_circle:',
      low: ':information_source:'
    };

    const color = {
      critical: '#FF0000',
      high: '#FFA500',
      medium: '#FFFF00',
      low: '#00FF00'
    };

    const payload = {
      channel: channel.channel || '#alerts',
      username: 'DevOps Alert Bot',
      icon_emoji: emoji[alert.severity] || ':bell:',
      attachments: [{
        color: color[alert.severity] || '#00FF00',
        title: `Alert: ${alert.name}`,
        text: alert.message,
        fields: [
          { title: 'Severity', value: alert.severity.toUpperCase(), short: true },
          { title: 'Source', value: alert.source || 'unknown', short: true }
        ],
        footer: 'DevOps Toolkit',
        ts: Math.floor(new Date(alert.timestamp).getTime() / 1000)
      }]
    };

    if (alert.metadata) {
      payload.attachments[0].fields.push({
        title: 'Details',
        value: JSON.stringify(alert.metadata).substring(0, 500),
        short: false
      });
    }

    return this.sendWebhook({
      url: channel.webhookUrl,
      headers: { 'Content-Type': 'application/json' }
    }, { ...alert, message: JSON.stringify(payload) });
  }

  // Send email
  async sendEmail(channel, alert) {
    // Email would require SMTP config - log for now
    console.log(`[AlertManager] Email alert: ${alert.name} to ${channel.recipients}`);
    return {
      success: true,
      reason: 'Email logged (SMTP not configured)'
    };
  }

  // Send to log
  sendToLog(channel, alert) {
    const level = alert.severity === 'critical' || alert.severity === 'high' ? 'error' : 'warn';
    console.log(`[ALERT:${alert.severity.toUpperCase()}] ${alert.name}: ${alert.message}`);
    return { success: true };
  }

  // Trigger an alert to all configured channels
  async triggerAlert(alert) {
    // Check rate limit
    if (!this.checkRateLimit(alert.name)) {
      console.log(`[AlertManager] Rate limited: ${alert.name}`);
      return { success: false, reason: 'Rate limited' };
    }

    // Add to history
    this.addToHistory(alert);

    const results = [];
    for (const [name, channel] of this.channels) {
      const result = await this.sendToChannel(channel, alert);
      results.push({ channel: name, ...result });
    }

    return {
      success: results.some(r => r.success),
      results
    };
  }

  // Add alert to history
  addToHistory(alert) {
    this.alertHistory.unshift({
      ...alert,
      receivedAt: new Date().toISOString()
    });

    // Trim history
    if (this.alertHistory.length > this.maxHistorySize) {
      this.alertHistory = this.alertHistory.slice(0, this.maxHistorySize);
    }
  }

  // Get alert history
  getHistory(options = {}) {
    let history = [...this.alertHistory];

    if (options.severity) {
      history = history.filter(a => a.severity === options.severity);
    }

    if (options.since) {
      const since = new Date(options.since).getTime();
      history = history.filter(a => new Date(a.timestamp).getTime() >= since);
    }

    if (options.limit) {
      history = history.slice(0, options.limit);
    }

    return history;
  }

  // Get alert statistics
  getStats() {
    const stats = {
      total: this.alertHistory.length,
      bySeverity: { critical: 0, high: 0, medium: 0, low: 0 },
      byChannel: {},
      last24h: 0
    };

    const dayAgo = Date.now() - 24 * 60 * 60 * 1000;

    for (const alert of this.alertHistory) {
      if (alert.severity) {
        stats.bySeverity[alert.severity]++;
      }
      if (new Date(alert.receivedAt).getTime() > dayAgo) {
        stats.last24h++;
      }
    }

    return stats;
  }

  // Configure from environment
  configureFromEnv() {
    // Slack channel
    if (process.env.ALERT_SLACK_WEBHOOK) {
      this.addChannel('slack', {
        type: 'slack',
        webhookUrl: process.env.ALERT_SLACK_WEBHOOK,
        channel: process.env.ALERT_SLACK_CHANNEL || '#alerts'
      });
    }

    // Generic webhook
    if (process.env.ALERT_WEBHOOK_URL) {
      this.addChannel('webhook', {
        type: 'webhook',
        url: process.env.ALERT_WEBHOOK_URL
      });
    }

    // Log-only (always enabled by default)
    this.addChannel('log', {
      type: 'log',
      enabled: true
    });
  }
}

// Singleton instance
const alertNotificationManager = new AlertNotificationManager();
alertNotificationManager.configureFromEnv();

module.exports = alertNotificationManager;