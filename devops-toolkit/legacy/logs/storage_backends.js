/**
 * Log Storage Backend Interface
 * Supports: Elasticsearch, Loki, and local file storage
 */

const fs = require('fs');
const path = require('path');
const http = require('http');
const https = require('https');

// Backend type constants
const BACKEND_TYPES = {
  LOCAL: 'local',
  ELASTICSEARCH: 'elasticsearch',
  LOKI: 'loki'
};

// Configuration for each backend
class StorageConfig {
  constructor() {
    this.type = process.env.LOG_STORAGE_BACKEND || BACKEND_TYPES.LOCAL;
    this.retention_days = parseInt(process.env.LOG_RETENTION_DAYS) || 30;
    this.es_config = {
      url: process.env.ELASTICSEARCH_URL || 'http://localhost:9200',
      index: process.env.ELASTICSEARCH_INDEX || 'devops-logs',
      username: process.env.ELASTICSEARCH_USERNAME || '',
      password: process.env.ELASTICSEARCH_PASSWORD || ''
    };
    this.loki_config = {
      url: process.env.LOKI_URL || 'http://localhost:3100',
      labels: {
        app: 'devops-toolkit',
        env: process.env.NODE_ENV || 'development'
      }
    };
  }
}

// Base interface for log storage backends
class LogStorageBackend {
  constructor(config) {
    this.config = config;
  }

  async initialize() {
    throw new Error('Not implemented');
  }

  async write(log) {
    throw new Error('Not implemented');
  }

  async query(options) {
    throw new Error('Not implemented');
  }

  async getStats() {
    throw new Error('Not implemented');
  }

  async deleteOldLogs(beforeDate) {
    throw new Error('Not implemented');
  }

  async healthCheck() {
    throw new Error('Not implemented');
  }
}

// Local file storage backend
class LocalStorageBackend extends LogStorageBackend {
  constructor(config) {
    super(config);
    this.storagePath = path.join(__dirname, '../config/logs.json');
    this.lockFile = path.join(__dirname, '../config/.logs.lock');
    this.data = { logs: [], alerts: [], filters: [] };
    this.load();
  }

  load() {
    try {
      if (fs.existsSync(this.storagePath)) {
        const content = fs.readFileSync(this.storagePath, 'utf8');
        this.data = JSON.parse(content);
      }
    } catch (e) {
      console.error('Failed to load local logs:', e.message);
    }
  }

  save() {
    try {
      fs.writeFileSync(this.storagePath, JSON.stringify(this.data, null, 2));
    } catch (e) {
      console.error('Failed to save local logs:', e.message);
    }
  }

  async initialize() {
    console.log('[LocalStorage] Initialized at', this.storagePath);
    return true;
  }

  async write(log) {
    this.data.logs.push(log);
    // Keep only retention_days of logs
    const cutoff = new Date();
    cutoff.setDate(cutoff.getDate() - this.config.retention_days);
    this.data.logs = this.data.logs.filter(l => new Date(l.timestamp) >= cutoff);
    this.save();
    return log;
  }

  async query(options = {}) {
    let results = [...this.data.logs];

    if (options.start_time) {
      results = results.filter(l => new Date(l.timestamp) >= new Date(options.start_time));
    }
    if (options.end_time) {
      results = results.filter(l => new Date(l.timestamp) <= new Date(options.end_time));
    }
    if (options.level) {
      results = results.filter(l => l.level === options.level);
    }
    if (options.source) {
      results = results.filter(l => l.source === options.source);
    }
    if (options.search) {
      const search = options.search.toLowerCase();
      results = results.filter(l => l.message.toLowerCase().includes(search));
    }

    const sortOrder = options.order === 'asc' ? 1 : -1;
    results.sort((a, b) => (new Date(a.timestamp) - new Date(b.timestamp)) * sortOrder);

    if (options.offset) results = results.slice(options.offset);
    if (options.limit) results = results.slice(0, options.limit);

    return { logs: results, total: results.length };
  }

  async getStats() {
    const now = Date.now();
    const oneHour = 60 * 60 * 1000;
    const oneDay = 24 * oneHour;

    const stats = {
      total: this.data.logs.length,
      by_level: {},
      by_source: {},
      last_hour: 0,
      last_24h: 0,
      error_rate: 0
    };

    for (const log of this.data.logs) {
      const time = new Date(log.timestamp).getTime();
      stats.by_level[log.level] = (stats.by_level[log.level] || 0) + 1;
      stats.by_source[log.source] = (stats.by_source[log.source] || 0) + 1;
      if (now - time < oneHour) stats.last_hour++;
      if (now - time < oneDay) stats.last_24h++;
    }

    const last24hLogs = this.data.logs.filter(l => now - new Date(l.timestamp).getTime() < oneDay);
    const errors = last24hLogs.filter(l => l.level === 'error').length;
    stats.error_rate = last24hLogs.length > 0 ? (errors / last24hLogs.length * 100).toFixed(2) : 0;

    return stats;
  }

  async deleteOldLogs(beforeDate) {
    const before = new Date(beforeDate);
    const beforeMs = before.getTime();
    const originalCount = this.data.logs.length;
    this.data.logs = this.data.logs.filter(l => new Date(l.timestamp).getTime() >= beforeMs);
    const deleted = originalCount - this.data.logs.length;
    if (deleted > 0) this.save();
    return { deleted };
  }

  async healthCheck() {
    return { healthy: true, backend: BACKEND_TYPES.LOCAL };
  }
}

// Elasticsearch backend
class ElasticsearchBackend extends LogStorageBackend {
  constructor(config) {
    super(config);
    this.es_config = config.es_config;
  }

  async initialize() {
    console.log('[Elasticsearch] Connecting to', this.es_config.url);
    try {
      await this.createIndexIfNotExists();
      return true;
    } catch (e) {
      console.error('[Elasticsearch] Init failed:', e.message);
      return false;
    }
  }

  async createIndexIfNotExists() {
    const indexName = this.es_config.index;
    const indexSettings = {
      index: {
        number_of_shards: 1,
        number_of_replicas: 0,
        'index.lifecycle.rollover_alias': 'devops-logs'
      }
    };

    try {
      await this.esRequest('HEAD', `/${indexName}`);
    } catch (e) {
      if (e.message.includes('404')) {
        await this.esRequest('PUT', `/${indexName}`, {
          mappings: {
            properties: {
              timestamp: { type: 'date' },
              level: { type: 'keyword' },
              message: { type: 'text' },
              source: { type: 'keyword' },
              resource: { type: 'keyword' },
              tags: { type: 'keyword' },
              metadata: { type: 'object', enabled: false }
            }
          },
          settings: indexSettings
        });
        console.log('[Elasticsearch] Index created:', indexName);
      }
    }
  }

  async esRequest(method, path, body = null) {
    return new Promise((resolve, reject) => {
      const url = new URL(this.es_config.url);
      const options = {
        hostname: url.hostname,
        port: url.port || (url.protocol === 'https:' ? 443 : 9200),
        path,
        method,
        headers: {
          'Content-Type': 'application/json'
        }
      };

      if (this.es_config.username && this.es_config.password) {
        const auth = Buffer.from(`${this.es_config.username}:${this.es_config.password}`).toString('base64');
        options.headers['Authorization'] = `Basic ${auth}`;
      }

      const protocol = url.protocol === 'https:' ? https : http;
      const req = protocol.request(options, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => {
          if (res.statusCode >= 400) {
            reject(new Error(`${res.statusCode}: ${data}`));
          } else {
            try {
              resolve(body ? JSON.parse(data) : {});
            } catch {
              resolve({});
            }
          }
        });
      });

      req.on('error', reject);
      if (body) req.write(JSON.stringify(body));
      req.end();
    });
  }

  async write(log) {
    const doc = {
      timestamp: log.timestamp || new Date().toISOString(),
      level: log.level,
      message: log.message,
      source: log.source,
      resource: log.resource,
      tags: log.tags || [],
      metadata: log.metadata || {}
    };

    await this.esRequest('POST', `/${this.es_config.index}/_doc`, doc);
    return log;
  }

  async query(options = {}) {
    const must = [];

    if (options.start_time || options.end_time) {
      const range = { timestamp: {} };
      if (options.start_time) range.timestamp.gte = options.start_time;
      if (options.end_time) range.timestamp.lte = options.end_time;
      must.push({ range });
    }
    if (options.level) must.push({ term: { level: options.level } });
    if (options.source) must.push({ term: { source: options.source } });
    if (options.search) must.push({ match: { message: options.search } });

    const query = must.length > 0 ? { bool: { must } } : { match_all: {} };

    const body = {
      query,
      sort: [{ timestamp: { order: options.order || 'desc' } }],
      from: options.offset || 0,
      size: options.limit || 100
    };

    const result = await this.esRequest('POST', `/${this.es_config.index}/_search`, body);

    return {
      logs: result.hits?.hits?.map(h => ({ id: h._id, ...h._source })) || [],
      total: result.hits?.total?.value || 0
    };
  }

  async getStats() {
    const result = await this.esRequest('POST', `/${this.es_config.index}/_search`, {
      query: { match_all: {} },
      aggs: {
        by_level: { terms: { field: 'level' } },
        by_source: { terms: { field: 'source' } }
      },
      size: 0
    });

    const aggs = result.aggregations || {};
    const stats = {
      total: result.hits?.total?.value || 0,
      by_level: {},
      by_source: {},
      last_hour: 0,
      last_24h: 0,
      error_rate: 0
    };

    for (const bucket of aggs.by_level?.buckets || []) {
      stats.by_level[bucket.key] = bucket.doc_count;
    }
    for (const bucket of aggs.by_source?.buckets || []) {
      stats.by_source[bucket.key] = bucket.doc_count;
    }

    return stats;
  }

  async deleteOldLogs(beforeDate) {
    const result = await this.esRequest('POST', `/${this.es_config.index}/_delete_by_query`, {
      query: {
        range: {
          timestamp: { lt: beforeDate }
        }
      }
    });
    return { deleted: result.deleted || 0 };
  }

  async healthCheck() {
    try {
      const info = await this.esRequest('GET', '/');
      return { healthy: true, backend: BACKEND_TYPES.ELASTICSEARCH, version: info.version?.number };
    } catch (e) {
      return { healthy: false, backend: BACKEND_TYPES.ELASTICSEARCH, error: e.message };
    }
  }
}

// Loki backend (using LogQL)
class LokiBackend extends LogStorageBackend {
  constructor(config) {
    super(config);
    this.loki_config = config.loki_config;
  }

  async initialize() {
    console.log('[Loki] Connecting to', this.loki_config.url);
    return true;
  }

  lokiRequest(path, body = null) {
    return new Promise((resolve, reject) => {
      const url = new URL(this.loki_config.url);
      const options = {
        hostname: url.hostname,
        port: url.port || 3100,
        path: `/loki/api/v1${path}`,
        method: body ? 'POST' : 'GET',
        headers: { 'Content-Type': 'application/json' }
      };

      const protocol = url.protocol === 'https:' ? https : http;
      const req = protocol.request(options, (res) => {
        let data = '';
        res.on('data', chunk => data += chunk);
        res.on('end', () => {
          if (res.statusCode >= 400) {
            reject(new Error(`${res.statusCode}: ${data}`));
          } else {
            try {
              resolve(JSON.parse(data));
            } catch {
              resolve({});
            }
          }
        });
      });

      req.on('error', reject);
      if (body) req.write(JSON.stringify(body));
      req.end();
    });
  }

  async write(log) {
    const labels = JSON.stringify({
      ...this.loki_config.labels,
      level: log.level,
      source: log.source,
      resource: log.resource || ''
    });

    const entry = {
      timestamp: (new Date(log.timestamp || Date.now()).getTime() * 1000000).toString(),
      line: `[${log.level.toUpperCase()}] ${log.source}: ${log.message}`
    };

    await this.lokiRequest('/push', {
      streams: [{
        labels,
        entries: [entry]
      }]
    });

    return log;
  }

  async query(options = {}) {
    const labels = [];
    if (options.level) labels.push(`level="${options.level}"`);
    if (options.source) labels.push(`source="${options.source}"`);

    let query = '{' + labels.join(',') + '}';
    if (options.search) {
      query += ` |= "${options.search}"`;
    }

    const limit = options.limit || 100;
    const direction = options.order === 'asc' ? 'BACKWARD' : 'FORWARD';

    const result = await this.lokiRequest('/query', {
      query,
      limit,
      direction
    });

    const logs = (result.data?.streams || []).flatMap(stream =>
      stream.entries.map(entry => ({
        timestamp: new Date(entry.timestamp / 1000000).toISOString(),
        message: entry.line,
        level: stream.labels?.level || 'info',
        source: stream.labels?.source || 'unknown'
      }))
    );

    return { logs, total: logs.length };
  }

  async getStats() {
    // Loki doesn't have easy aggregation, return basic stats
    return {
      total: 0,
      by_level: { info: 0 },
      by_source: {},
      last_hour: 0,
      last_24h: 0,
      error_rate: 0
    };
  }

  async deleteOldLogs(beforeDate) {
    // Loki retention is typically handled by configuration
    return { deleted: 0, note: 'Loki retention handled by configuration' };
  }

  async healthCheck() {
    try {
      await this.lokiRequest('/version');
      return { healthy: true, backend: BACKEND_TYPES.LOKI };
    } catch (e) {
      return { healthy: false, backend: BACKEND_TYPES.LOKI, error: e.message };
    }
  }
}

// Factory to create storage backend
function createStorageBackend(type = BACKEND_TYPES.LOCAL, config = new StorageConfig()) {
  switch (type || config.type) {
    case BACKEND_TYPES.ELASTICSEARCH:
      return new ElasticsearchBackend(config);
    case BACKEND_TYPES.LOKI:
      return new LokiBackend(config);
    default:
      return new LocalStorageBackend(config);
  }
}

module.exports = {
  BACKEND_TYPES,
  StorageConfig,
  LogStorageBackend,
  LocalStorageBackend,
  ElasticsearchBackend,
  LokiBackend,
  createStorageBackend
};
