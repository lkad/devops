/**
 * Log Manager
 * Manages log storage, indexing, and querying based on DESIGN.md Section 3
 * Supports multiple storage backends: Local (JSON), Elasticsearch, Loki
 */

const fs = require('fs');
const path = require('path');
const { v4: uuidv4 } = require('uuid');
const { createStorageBackend, StorageConfig, BACKEND_TYPES } = require('./storage_backends');

class LogManager {
  constructor(storagePath) {
    this.storagePath = storagePath || path.join(__dirname, '../config/logs.json');
    this.logs = [];
    this.alerts = [];
    this.filters = [];

    // Initialize storage backend from environment config
    const config = new StorageConfig();
    this.backend = createStorageBackend(config.type, config);
    this.retentionDays = config.retention_days;

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

    // Write to storage backend (all backends)
    this.backend.write(log).catch(e => {
      console.error('Failed to write to backend:', e.message);
    });

    // For Local backend: also maintain local array for operational logs
    // For ES/Loki backend: backend IS the source, don't duplicate locally
    if (this.backend.constructor.name === 'LocalStorageBackend') {
      this.logs.push(log);
    }

    // Check alert rules
    this.checkAlerts(log);

    // Save periodically (every 10 logs) - only for local backend
    if (this.backend.constructor.name === 'LocalStorageBackend' && this.logs.length % 10 === 0) {
      this.save();
    }

    // Apply retention policy periodically
    if (this.logs.length % 100 === 0) {
      this.applyRetention();
    }

    return log;
  }

  // Query logs - delegates based on backend type
  queryLogs(options = {}) {
    const backendType = this.backend.constructor.name;

    if (backendType === 'LocalStorageBackend') {
      // Local backend: query from local array (operational logs only)
      return this.queryLogsLocal(options);
    } else {
      // ES/Loki backend: query from backend API (includes external logs)
      // Synchronous wrapper for async backend query
      // Note: For production, server.js should use queryLogsBackend() instead
      return this.queryLogsFromBackend(options);
    }
  }

  // Query from local array
  queryLogsLocal(options = {}) {
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

  // Query logs using storage backend (Elasticsearch/Loki)
  async queryLogsBackend(options = {}) {
    try {
      const result = await this.backend.query(options);
      return result;
    } catch (e) {
      console.error('Backend query failed:', e.message);
      return { logs: [], total: 0, has_more: false };
    }
  }

  // Synchronous wrapper for backend query (for compatibility)
  queryLogsFromBackend(options = {}) {
    // Returns sync result structure, actual query happens async
    // Server should use queryLogsBackend() for proper async handling
    return { logs: [], total: 0, has_more: false, note: 'use queryLogsBackend() for ES/Loki' };
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
      error_rate: 0,
      backend: this.backend.constructor.name.replace('Backend', '').toLowerCase()
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

  // Get stats from backend
  async getBackendStats() {
    try {
      return await this.backend.getStats();
    } catch (e) {
      console.error('Backend stats failed:', e.message);
      return this.getLogStats();
    }
  }

  // ===========================================
  // Log Retention Management
  // ===========================================

  /**
   * Apply retention policy - delete logs older than retention_days
   * @returns {Promise<{deleted: number, retention_days: number}>}
   */
  async applyRetention() {
    const cutoff = new Date();
    cutoff.setDate(cutoff.getDate() - this.retentionDays);
    const beforeMs = cutoff.getTime();

    const originalCount = this.logs.length;
    this.logs = this.logs.filter(l => new Date(l.timestamp).getTime() >= beforeMs);
    const deleted = originalCount - this.logs.length;

    if (deleted > 0) {
      this.save();
      console.log(`[LogManager] Retention cleanup: deleted ${deleted} logs older than ${this.retentionDays} days`);

      // Also cleanup in backend storage
      try {
        await this.backend.deleteOldLogs(cutoff.toISOString());
      } catch (e) {
        console.error('Backend retention cleanup failed:', e.message);
      }
    }

    return { deleted, retention_days: this.retentionDays };
  }

  /**
   * Get retention configuration
   * @returns {Object} Retention config
   */
  getRetentionConfig() {
    return {
      retention_days: this.retentionDays,
      backend: this.backend.constructor.name.replace('Backend', '').toLowerCase(),
      backend_type: BACKEND_TYPES[this.backend.constructor.name.replace('Backend', '').toUpperCase()] || 'local'
    };
  }

  /**
   * Update retention configuration
   * @param {Object} config - New retention config
   * @returns {Object} Updated retention config
   */
  updateRetentionConfig(config) {
    if (config.retention_days !== undefined) {
      this.retentionDays = Math.max(1, Math.min(3650, parseInt(config.retention_days) || 30));
    }
    return this.getRetentionConfig();
  }

  /**
   * Get storage backend health status
   * @returns {Promise<{healthy: boolean, backend: string, version?: string, error?: string}>}
   */
  async getBackendHealth() {
    try {
      return await this.backend.healthCheck();
    } catch (e) {
      return { healthy: false, backend: 'unknown', error: e.message };
    }
  }

  // ===========================================
  // Alert Management
  // ===========================================

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

  // Initialize backend
  async initialize() {
    try {
      await this.backend.initialize();
      return true;
    } catch (e) {
      console.error('Backend initialization failed:', e.message);
      return false;
    }
  }
}

module.exports = LogManager;