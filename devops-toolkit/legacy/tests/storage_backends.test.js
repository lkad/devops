/**
 * Tests for Storage Backends
 * Covers: Local, Elasticsearch, Loki backends
 */

describe('StorageConfig', () => {
  let StorageConfig, BACKEND_TYPES;

  beforeEach(() => {
    jest.resetModules();
    // Set environment before requiring
    process.env.LOG_STORAGE_BACKEND = 'local';
    const module = require('../logs/storage_backends');
    StorageConfig = module.StorageConfig;
    BACKEND_TYPES = module.BACKEND_TYPES;
  });

  afterEach(() => {
    delete process.env.LOG_STORAGE_BACKEND;
  });

  it('should have backend types defined', () => {
    expect(BACKEND_TYPES).toBeDefined();
    expect(BACKEND_TYPES.LOCAL).toBe('local');
    expect(BACKEND_TYPES.ELASTICSEARCH).toBe('elasticsearch');
    expect(BACKEND_TYPES.LOKI).toBe('loki');
  });

  it('should create config with defaults', () => {
    const config = new StorageConfig();
    expect(config.type).toBe('local');
    expect(config.retention_days).toBe(30);
  });

  it('should read from environment', () => {
    process.env.LOG_STORAGE_BACKEND = 'elasticsearch';
    process.env.LOG_RETENTION_DAYS = '60';

    jest.resetModules();
    const module = require('../logs/storage_backends');
    const config = new module.StorageConfig();

    expect(config.type).toBe('elasticsearch');
    expect(config.retention_days).toBe(60);
  });
});

describe('LocalStorageBackend', () => {
  let createStorageBackend, StorageConfig, BACKEND_TYPES, LocalStorageBackend;
  const fs = require('fs');
  const path = require('path');
  const testPath = '/tmp/test-storage-backend.json';

  beforeEach(() => {
    jest.resetModules();
    process.env.LOG_STORAGE_BACKEND = 'local';
    const module = require('../logs/storage_backends');
    createStorageBackend = module.createStorageBackend;
    StorageConfig = module.StorageConfig;
    BACKEND_TYPES = module.BACKEND_TYPES;
    LocalStorageBackend = module.LocalStorageBackend;
    if (fs.existsSync(testPath)) fs.unlinkSync(testPath);
  });

  afterEach(() => {
    if (fs.existsSync(testPath)) fs.unlinkSync(testPath);
  });

  it('should create local backend', () => {
    const config = new StorageConfig();
    const backend = createStorageBackend(BACKEND_TYPES.LOCAL, config);
    expect(backend).toBeDefined();
    expect(backend.write).toBeDefined();
    expect(backend.query).toBeDefined();
  });

  it('should have LocalStorageBackend class', () => {
    expect(LocalStorageBackend).toBeDefined();
  });

  it('should create backend with custom storage path', () => {
    const config = new StorageConfig();
    config.storage_path = '/tmp/test-logs.json';
    const backend = new LocalStorageBackend(config);
    expect(backend).toBeDefined();
  });

  describe('write method', () => {
    it('should write a log entry', async () => {
      const config = new StorageConfig();
      config.retention_days = 30;
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      backend.data = { logs: [], alerts: [], filters: [] };

      const log = { id: '1', timestamp: new Date().toISOString(), level: 'info', message: 'test', source: 'test' };
      const result = await backend.write(log);

      expect(result).toBe(log);
      expect(backend.data.logs).toContain(log);
    });

    it('should apply retention on write', async () => {
      const config = new StorageConfig();
      config.retention_days = 7;
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      backend.data = { logs: [], alerts: [], filters: [] };

      const oldLog = { id: '1', timestamp: new Date(Date.now() - 100 * 24 * 3600000).toISOString(), level: 'info', message: 'old', source: 'test' };
      await backend.write(oldLog);
      expect(backend.data.logs).toHaveLength(0);
    });
  });

  describe('query method', () => {
    let backend;

    beforeEach(() => {
      const config = new StorageConfig();
      backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      const now = Date.now();
      backend.data = {
        logs: [
          { id: '1', timestamp: new Date(now - 3600000).toISOString(), level: 'info', message: 'info msg', source: 'web' },
          { id: '2', timestamp: new Date(now - 7200000).toISOString(), level: 'error', message: 'error msg', source: 'api' },
          { id: '3', timestamp: new Date(now).toISOString(), level: 'warn', message: 'warn msg', source: 'web' }
        ],
        alerts: [],
        filters: []
      };
    });

    it('should return all logs', async () => {
      const result = await backend.query({});
      expect(result.logs).toHaveLength(3);
    });

    it('should filter by level', async () => {
      const result = await backend.query({ level: 'error' });
      expect(result.logs.every(l => l.level === 'error')).toBe(true);
    });

    it('should filter by source', async () => {
      const result = await backend.query({ source: 'web' });
      expect(result.logs.every(l => l.source === 'web')).toBe(true);
    });

    it('should filter by search', async () => {
      const result = await backend.query({ search: 'error' });
      expect(result.logs.every(l => l.message.toLowerCase().includes('error'))).toBe(true);
    });

    it('should sort ascending', async () => {
      const result = await backend.query({ order: 'asc' });
      expect(new Date(result.logs[0].timestamp).getTime()).toBeLessThanOrEqual(
        new Date(result.logs[result.logs.length - 1].timestamp).getTime()
      );
    });

    it('should apply limit', async () => {
      const result = await backend.query({ limit: 2 });
      expect(result.logs).toHaveLength(2);
    });

    it('should apply offset', async () => {
      const result = await backend.query({ offset: 1 });
      expect(result.logs.length).toBe(2); // 3 logs - 1 offset = 2
    });
  });

  describe('getStats method', () => {
    it('should return stats', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      const now = Date.now();
      backend.data = {
        logs: [
          { id: '1', timestamp: new Date(now - 30 * 60000).toISOString(), level: 'info', message: 'a', source: 'web' },
          { id: '2', timestamp: new Date(now - 2 * 3600000).toISOString(), level: 'error', message: 'b', source: 'api' }
        ],
        alerts: [],
        filters: []
      };

      const stats = await backend.getStats();
      expect(stats.total).toBe(2);
      expect(stats.by_level.info).toBe(1);
      expect(stats.by_level.error).toBe(1);
      expect(stats.last_hour).toBe(1);
      expect(stats.last_24h).toBe(2);
    });

    it('should calculate error rate', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      const now = Date.now();
      backend.data = {
        logs: [
          { id: '1', timestamp: new Date(now - 30 * 60000).toISOString(), level: 'info', message: 'a', source: 'web' },
          { id: '2', timestamp: new Date(now - 30 * 60000).toISOString(), level: 'error', message: 'b', source: 'web' }
        ],
        alerts: [],
        filters: []
      };

      const stats = await backend.getStats();
      expect(parseFloat(stats.error_rate)).toBe(50);
    });
  });

  describe('deleteOldLogs method', () => {
    it('should delete old logs', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      const now = Date.now();
      backend.data = {
        logs: [
          { id: '1', timestamp: new Date(now - 100 * 24 * 3600000).toISOString(), level: 'info', message: 'old', source: 'web' },
          { id: '2', timestamp: new Date().toISOString(), level: 'info', message: 'new', source: 'web' }
        ],
        alerts: [],
        filters: []
      };

      const cutoff = new Date(now - 60 * 24 * 3600000).toISOString();
      const result = await backend.deleteOldLogs(cutoff);
      expect(result.deleted).toBe(1);
      expect(backend.data.logs).toHaveLength(1);
    });

    it('should return 0 when nothing to delete', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      backend.storagePath = testPath;
      backend.data = {
        logs: [{ id: '1', timestamp: new Date().toISOString(), level: 'info', message: 'new', source: 'web' }],
        alerts: [],
        filters: []
      };

      const cutoff = new Date(Date.now() - 60 * 24 * 3600000).toISOString();
      const result = await backend.deleteOldLogs(cutoff);
      expect(result.deleted).toBe(0);
    });
  });

  describe('healthCheck method', () => {
    it('should return healthy', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      const result = await backend.healthCheck();
      expect(result.healthy).toBe(true);
      expect(result.backend).toBe('local');
    });
  });

  describe('initialize method', () => {
    it('should return true', async () => {
      const config = new StorageConfig();
      const backend = new LocalStorageBackend(config);
      const result = await backend.initialize();
      expect(result).toBe(true);
    });
  });
});

describe('ElasticsearchBackend', () => {
  let createStorageBackend, StorageConfig, BACKEND_TYPES;

  beforeEach(() => {
    jest.resetModules();
    const module = require('../logs/storage_backends');
    createStorageBackend = module.createStorageBackend;
    StorageConfig = module.StorageConfig;
    BACKEND_TYPES = module.BACKEND_TYPES;
  });

  it('should create elasticsearch backend', () => {
    const config = new StorageConfig();
    config.type = BACKEND_TYPES.ELASTICSEARCH;
    const backend = createStorageBackend(BACKEND_TYPES.ELASTICSEARCH, config);
    expect(backend).toBeDefined();
  });
});

describe('LokiBackend', () => {
  let createStorageBackend, StorageConfig, BACKEND_TYPES;

  beforeEach(() => {
    jest.resetModules();
    const module = require('../logs/storage_backends');
    createStorageBackend = module.createStorageBackend;
    StorageConfig = module.StorageConfig;
    BACKEND_TYPES = module.BACKEND_TYPES;
  });

  it('should create loki backend', () => {
    const config = new StorageConfig();
    config.type = BACKEND_TYPES.LOKI;
    const backend = createStorageBackend(BACKEND_TYPES.LOKI, config);
    expect(backend).toBeDefined();
  });
});

describe('createStorageBackend factory', () => {
  let createStorageBackend, StorageConfig, BACKEND_TYPES;

  beforeEach(() => {
    jest.resetModules();
    const module = require('../logs/storage_backends');
    createStorageBackend = module.createStorageBackend;
    StorageConfig = module.StorageConfig;
    BACKEND_TYPES = module.BACKEND_TYPES;
  });

  it('should create local backend', () => {
    const backend = createStorageBackend(BACKEND_TYPES.LOCAL, new StorageConfig());
    expect(backend.constructor.name).toBe('LocalStorageBackend');
  });

  it('should create elasticsearch backend', () => {
    const backend = createStorageBackend(BACKEND_TYPES.ELASTICSEARCH, new StorageConfig());
    expect(backend.constructor.name).toBe('ElasticsearchBackend');
  });

  it('should create loki backend', () => {
    const backend = createStorageBackend(BACKEND_TYPES.LOKI, new StorageConfig());
    expect(backend.constructor.name).toBe('LokiBackend');
  });

  it('should default to local for unknown type', () => {
    const backend = createStorageBackend('unknown', new StorageConfig());
    expect(backend.constructor.name).toBe('LocalStorageBackend');
  });
});
