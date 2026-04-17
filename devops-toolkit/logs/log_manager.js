/**
 * Log Manager
 * Manages log storage, indexing, and querying based on DESIGN.md Section 3
 */

const fs = require('fs');
const path = require('path');
const { v4: uuidv4 } = require('uuid');

class LogManager {
  constructor(storagePath) {
    this.storagePath = storagePath || path.join(__dirname, '../config/logs.json');
    this.logs = [];
    this.alerts = [];
    this.filters = [];
    this.load();
  }

  load() {
    try {
      if (fs.existsSync(this.storagePath)) {
        const data = JSON.parse(fs.readFileSync(this.storagePath, 'utf8'));
        this.logs = data.logs || [];
        this.alerts = data.alerts || [];
        this.filters = data.filters || [];
      }
    } catch (e) {
      console.error('Failed to load logs:', e.message);
    }
  }

  save() {
    try {
      const data = {
        logs: this.logs.slice(-10000), // Keep last 10000 logs
        alerts: this.alerts,
        filters: this.filters
      };
      fs.writeFileSync(this.storagePath, JSON.stringify(data, null, 2));
    } catch (e) {
      console.error('Failed to save logs:', e.message);
    }
  }

  // Add a log entry
  addLog(entry) {
    const log = {
      id: uuidv4(),
      timestamp: entry.timestamp || new Date().toISOString(),
      level: entry.level || 'info', // debug, info, warn, error
      message: entry.message,
      source: entry.source || 'unknown',
      resource: entry.resource || null, // device_id, service name, etc.
      metadata: entry.metadata || {},
      tags: entry.tags || []
    };

    this.logs.push(log);

    // Check alert rules
    this.checkAlerts(log);

    // Save periodically (every 10 logs)
    if (this.logs.length % 10 === 0) {
      this.save();
    }

    return log;
  }

  // Query logs
  queryLogs(options = {}) {
    let results = [...this.logs];

    // Filter by time range
    if (options.start_time) {
      const start = new Date(options.start_time);
      results = results.filter(l => new Date(l.timestamp) >= start);
    }
    if (options.end_time) {
      const end = new Date(options.end_time);
      results = results.filter(l => new Date(l.timestamp) <= end);
    }

    // Filter by level
    if (options.level) {
      results = results.filter(l => l.level === options.level);
    }
    if (options.levels && options.levels.length > 0) {
      results = results.filter(l => options.levels.includes(l.level));
    }

    // Filter by source
    if (options.source) {
      results = results.filter(l => l.source === options.source);
    }

    // Filter by resource
    if (options.resource) {
      results = results.filter(l => l.resource === options.resource);
    }

    // Filter by message content (search)
    if (options.search) {
      const searchLower = options.search.toLowerCase();
      results = results.filter(l =>
        l.message.toLowerCase().includes(searchLower) ||
        (l.source && l.source.toLowerCase().includes(searchLower))
      );
    }

    // Filter by tags
    if (options.tags && options.tags.length > 0) {
      results = results.filter(l =>
        options.tags.some(tag => l.tags.includes(tag))
      );
    }

    // Sort (newest first by default)
    const sortOrder = options.order === 'asc' ? 1 : -1;
    results.sort((a, b) => {
      const timeA = new Date(a.timestamp).getTime();
      const timeB = new Date(b.timestamp).getTime();
      return (timeA - timeB) * sortOrder;
    });

    // Pagination
    const total = results.length;
    if (options.offset) {
      results = results.slice(options.offset);
    }
    if (options.limit) {
      results = results.slice(0, options.limit);
    }

    return {
      logs: results,
      total,
      has_more: options.offset + options.limit < total
    };
  }

  // Get logs by resource (device, service, etc.)
  getLogsByResource(resource, limit = 100) {
    return this.queryLogs({ resource, limit, order: 'desc' });
  }

  // Get recent logs
  getRecentLogs(limit = 100) {
    return this.queryLogs({ limit, order: 'desc' });
  }

  // Get log statistics
  getLogStats() {
    const now = Date.now();
    const oneHour = 60 * 60 * 1000;
    const oneDay = 24 * oneHour;

    const counts = {
      total: this.logs.length,
      by_level: {},
      by_source: {},
      last_hour: 0,
      last_24h: 0,
      error_rate: 0
    };

    for (const log of this.logs) {
      const time = new Date(log.timestamp).getTime();

      // Count by level
      counts.by_level[log.level] = (counts.by_level[log.level] || 0) + 1;

      // Count by source
      counts.by_source[log.source] = (counts.by_source[log.source] || 0) + 1;

      // Count recent
      if (now - time < oneHour) counts.last_hour++;
      if (now - time < oneDay) counts.last_24h++;
    }

    // Calculate error rate (errors / total in last 24h)
    const last24hLogs = this.logs.filter(l =>
      now - new Date(l.timestamp).getTime() < oneDay
    );
    const errorCount = last24hLogs.filter(l => l.level === 'error').length;
    counts.error_rate = last24hLogs.length > 0
      ? (errorCount / last24hLogs.length * 100).toFixed(2)
      : 0;

    return counts;
  }

  // Alert management
  createAlertRule(rule) {
    const alertRule = {
      id: uuidv4(),
      name: rule.name,
      pattern: rule.pattern || '', // regex or logQL-like pattern
      level: rule.level || 'error',
      source: rule.source || null,
      threshold: rule.threshold || 1, // count within window
      window_ms: rule.window_ms || 60000, // time window in ms
      enabled: rule.enabled !== false,
      created_at: new Date().toISOString()
    };

    this.alerts.push(alertRule);
    this.save();
    return alertRule;
  }

  getAlertRules() {
    return this.alerts;
  }

  updateAlertRule(id, updates) {
    const index = this.alerts.findIndex(a => a.id === id);
    if (index === -1) return null;

    this.alerts[index] = { ...this.alerts[index], ...updates, id };
    this.save();
    return this.alerts[index];
  }

  deleteAlertRule(id) {
    const index = this.alerts.findIndex(a => a.id === id);
    if (index === -1) return false;

    this.alerts.splice(index, 1);
    this.save();
    return true;
  }

  // Check logs against alert rules
  checkAlerts(log) {
    for (const rule of this.alerts) {
      if (!rule.enabled) continue;

      let matched = false;

      // Level match
      if (rule.level === log.level) {
        // Pattern match
        if (rule.pattern) {
          try {
            const regex = new RegExp(rule.pattern, 'i');
            matched = regex.test(log.message);
          } catch (e) {
            matched = log.message.includes(rule.pattern);
          }
        } else {
          matched = true;
        }

        // Source match
        if (rule.source && rule.source !== log.source) {
          matched = false;
        }
      }

      if (matched) {
        // In a real system, this would trigger notifications
        // For now, just log that the alert was triggered
        console.log(`[ALERT] Rule "${rule.name}" triggered by log: ${log.message}`);
      }
    }
  }

  // Saved filters
  saveFilter(filter) {
    const savedFilter = {
      id: filter.id || uuidv4(),
      name: filter.name,
      query: filter.query || {},
      created_at: new Date().toISOString()
    };

    const existing = this.filters.findIndex(f => f.id === savedFilter.id);
    if (existing >= 0) {
      this.filters[existing] = savedFilter;
    } else {
      this.filters.push(savedFilter);
    }

    this.save();
    return savedFilter;
  }

  getSavedFilters() {
    return this.filters;
  }

  deleteFilter(id) {
    const index = this.filters.findIndex(f => f.id === id);
    if (index === -1) return false;

    this.filters.splice(index, 1);
    this.save();
    return true;
  }

  // Generate sample logs for demo
  generateSampleLogs(count = 50) {
    const levels = ['debug', 'info', 'warn', 'error'];
    const sources = ['web-server', 'api-gateway', 'database', 'auth-service', 'device-agent'];
    const messages = {
      debug: ['Debug info: cache miss', 'Debug: connection pool size 10', 'Debug: request queued'],
      info: ['Request processed', 'Service started', 'Connection established', 'Health check passed'],
      warn: ['High memory usage detected', 'Slow query detected', 'Connection pool near capacity'],
      error: ['Connection refused', 'Authentication failed', 'Request timeout', 'Service unavailable']
    };

    for (let i = 0; i < count; i++) {
      const level = levels[Math.floor(Math.random() * levels.length)];
      const source = sources[Math.floor(Math.random() * sources.length)];
      const msgs = messages[level];

      this.addLog({
        level,
        source,
        message: msgs[Math.floor(Math.random() * msgs.length)],
        resource: `device-${Math.floor(Math.random() * 5) + 1}`,
        timestamp: new Date(Date.now() - Math.random() * 3600000).toISOString(),
        tags: [source, `env:${['dev', 'prod'][Math.floor(Math.random() * 2)]}`]
      });
    }

    this.save();
    return { generated: count };
  }
}

module.exports = LogManager;
