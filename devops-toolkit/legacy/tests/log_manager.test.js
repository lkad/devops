/**
 * Tests for LogManager
 * Tests log storage, querying, alert rules, and retention
 */

const path = require('path');
const fs = require('fs');

// UUID counter for deterministic IDs (must be before jest.mock calls)
let mockUuidCounter = 0;

// Mock storage_backends before requiring LogManager
jest.mock('../logs/storage_backends', () => {
  const BACKEND_TYPES = { LOCAL: 'local', ELASTICSEARCH: 'elasticsearch', LOKI: 'loki' };

  class LocalStorageBackend {
    constructor() {
      this.name = 'LocalStorageBackend';
    }
    async write(log) {
      return true;
    }
    async query(options) {
      return { logs: [], total: 0, has_more: false };
    }
    async getStats() {
      return { total: 0 };
    }
    async healthCheck() {
      return { healthy: true, backend: 'mock' };
    }
    async deleteOldLogs(cutoff) {
      return 0;
    }
  }

  function createStorageBackend(type, config) {
    return new LocalStorageBackend();
  }

  class StorageConfig {
    constructor() {
      this.type = 'local';
      this.retention_days = 7;
    }
  }

  return { createStorageBackend, StorageConfig, BACKEND_TYPES };
});

// Mock uuid for deterministic IDs
jest.mock('uuid', () => ({
  v4: () => `test-uuid-${++mockUuidCounter}`
}));

const LogManager = require('../logs/log_manager');

describe('LogManager', () => {
  let manager;
  const testStoragePath = path.join(__dirname, '../config/test-logs.json');

  beforeEach(() => {
    // Reset UUID counter for each test
    mockUuidCounter = 0;
    // Clear the file if it exists
    if (fs.existsSync(testStoragePath)) {
      fs.unlinkSync(testStoragePath);
    }
    manager = new LogManager(testStoragePath);
    // Clear logs array for each test (since LocalStorageBackend pushes to logs)
    manager.logs = [];
    manager.alerts = [];
    manager.filters = [];
  });

  afterEach(() => {
    if (fs.existsSync(testStoragePath)) {
      fs.unlinkSync(testStoragePath);
    }
  });

  describe('Constructor', () => {
    it('should create with storage path', () => {
      expect(manager.storagePath).toBe(testStoragePath);
      expect(manager.backend).toBeDefined();
      expect(manager.retentionDays).toBeDefined();
    });

    it('should have null callbacks initially', () => {
      expect(manager.onLogAdded).toBeNull();
      expect(manager.onAlertTriggered).toBeNull();
    });
  });

  describe('setCallbacks', () => {
    it('should set callbacks', () => {
      const cb1 = jest.fn();
      const cb2 = jest.fn();
      manager.setCallbacks(cb1, cb2);
      expect(manager.onLogAdded).toBe(cb1);
      expect(manager.onAlertTriggered).toBe(cb2);
    });
  });

  describe('addLog', () => {
    it('should add a log entry', () => {
      const log = manager.addLog({
        message: 'Test message',
        level: 'info',
        source: 'test'
      });

      expect(log.id).toBe('test-uuid-1');
      expect(log.message).toBe('Test message');
      expect(log.level).toBe('info');
      expect(log.source).toBe('test');
      expect(log.timestamp).toBeDefined();
    });

    it('should use default values', () => {
      const log = manager.addLog({ message: 'Test' });
      expect(log.level).toBe('info');
      expect(log.source).toBe('unknown');
      expect(log.resource).toBeNull();
      expect(log.metadata).toEqual({});
      expect(log.tags).toEqual([]);
    });

    it('should call onLogAdded callback', () => {
      const cb = jest.fn();
      manager.onLogAdded = cb;
      manager.addLog({ message: 'Callback test' });
      expect(cb).toHaveBeenCalled();
    });

    it('should trigger alert check', () => {
      // Create an alert rule first
      manager.createAlertRule({
        name: 'Test Alert',
        level: 'error',
        pattern: '',
        threshold: 1
      });
      const alertCb = jest.fn();
      manager.onAlertTriggered = alertCb;

      // Add a log that should trigger the alert
      manager.addLog({ message: 'Some error', level: 'error', source: 'test' });
      expect(alertCb).toHaveBeenCalled();
    });

    it('should not trigger alert for non-matching level', () => {
      manager.createAlertRule({
        name: 'Test Alert',
        level: 'error',
        threshold: 1
      });
      const alertCb = jest.fn();
      manager.onAlertTriggered = alertCb;

      manager.addLog({ message: 'Info message', level: 'info', source: 'test' });
      expect(alertCb).not.toHaveBeenCalled();
    });
  });

  describe('queryLogsLocal', () => {
    beforeEach(() => {
      // Add some test logs
      for (let i = 0; i < 5; i++) {
        manager.logs.push({
          id: `log-${i}`,
          timestamp: new Date(Date.now() - i * 1000).toISOString(),
          level: i % 2 === 0 ? 'info' : 'error',
          message: `Message ${i}`,
          source: i % 2 === 0 ? 'web' : 'api',
          resource: `device-${i}`,
          metadata: {},
          tags: ['tag1']
        });
      }
    });

    it('should return all logs by default', () => {
      const result = manager.queryLogsLocal({});
      expect(result.logs).toHaveLength(5);
    });

    it('should filter by level', () => {
      const result = manager.queryLogsLocal({ level: 'error' });
      expect(result.logs.every(l => l.level === 'error')).toBe(true);
    });

    it('should filter by multiple levels', () => {
      const result = manager.queryLogsLocal({ levels: ['error', 'warn'] });
      expect(result.logs.every(l => ['error', 'warn'].includes(l.level))).toBe(true);
    });

    it('should filter by source', () => {
      const result = manager.queryLogsLocal({ source: 'web' });
      expect(result.logs.every(l => l.source === 'web')).toBe(true);
    });

    it('should filter by resource', () => {
      const result = manager.queryLogsLocal({ resource: 'device-0' });
      expect(result.logs).toHaveLength(1);
      expect(result.logs[0].resource).toBe('device-0');
    });

    it('should filter by search term', () => {
      manager.logs[0].message = 'special search term';
      const result = manager.queryLogsLocal({ search: 'search' });
      expect(result.logs.length).toBeGreaterThan(0);
    });

    it('should filter by tags', () => {
      const result = manager.queryLogsLocal({ tags: ['tag1'] });
      expect(result.logs.every(l => l.tags.includes('tag1'))).toBe(true);
    });

    it('should sort descending by default', () => {
      const result = manager.queryLogsLocal({});
      expect(new Date(result.logs[0].timestamp).getTime()).toBeGreaterThan(
        new Date(result.logs[result.logs.length - 1].timestamp).getTime()
      );
    });

    it('should sort ascending when specified', () => {
      const result = manager.queryLogsLocal({ order: 'asc' });
      expect(new Date(result.logs[0].timestamp).getTime()).toBeLessThan(
        new Date(result.logs[result.logs.length - 1].timestamp).getTime()
      );
    });

    it('should limit results', () => {
      const result = manager.queryLogsLocal({ limit: 2 });
      expect(result.logs).toHaveLength(2);
    });

    it('should offset results', () => {
      const result = manager.queryLogsLocal({ offset: 2 });
      expect(result.total).toBe(5);
    });

    it('should return total count', () => {
      const result = manager.queryLogsLocal({ limit: 2 });
      expect(result.total).toBe(5);
    });
  });

  describe('getLogsByResource', () => {
    it('should return logs for resource', () => {
      manager.logs = [
        { id: '1', timestamp: new Date().toISOString(), level: 'info', message: 'a', source: 's', resource: 'res1', metadata: {}, tags: [] },
        { id: '2', timestamp: new Date().toISOString(), level: 'info', message: 'b', source: 's', resource: 'res2', metadata: {}, tags: [] }
      ];
      const result = manager.getLogsByResource('res1');
      expect(result.logs).toHaveLength(1);
    });
  });

  describe('getRecentLogs', () => {
    it('should return recent logs', () => {
      manager.logs = [
        { id: '1', timestamp: new Date().toISOString(), level: 'info', message: 'a', source: 's', resource: null, metadata: {}, tags: [] },
        { id: '2', timestamp: new Date().toISOString(), level: 'info', message: 'b', source: 's', resource: null, metadata: {}, tags: [] }
      ];
      const result = manager.getRecentLogs(1);
      expect(result.logs).toHaveLength(1);
    });
  });

  describe('getLogStats', () => {
    it('should return stats object', () => {
      manager.logs = [
        { id: '1', timestamp: new Date().toISOString(), level: 'info', message: 'a', source: 'web', resource: null, metadata: {}, tags: [] },
        { id: '2', timestamp: new Date().toISOString(), level: 'error', message: 'b', source: 'api', resource: null, metadata: {}, tags: [] }
      ];
      const stats = manager.getLogStats();
      expect(stats.total).toBe(2);
      expect(stats.by_level.info).toBe(1);
      expect(stats.by_level.error).toBe(1);
      expect(stats.by_source.web).toBe(1);
      expect(stats.by_source.api).toBe(1);
      expect(stats.backend).toBeDefined();
    });

    it('should count last hour logs', () => {
      const now = Date.now();
      manager.logs = [
        { id: '1', timestamp: new Date(now - 30 * 60000).toISOString(), level: 'info', message: 'a', source: 'web', resource: null, metadata: {}, tags: [] },
        { id: '2', timestamp: new Date(now - 2 * 3600000).toISOString(), level: 'info', message: 'b', source: 'web', resource: null, metadata: {}, tags: [] }
      ];
      const stats = manager.getLogStats();
      expect(stats.last_hour).toBe(1);
      expect(stats.last_24h).toBe(2);
    });
  });

  describe('Alert Rules', () => {
    describe('createAlertRule', () => {
      it('should create alert rule with defaults', () => {
        const rule = manager.createAlertRule({ name: 'Test Rule' });
        expect(rule.name).toBe('Test Rule');
        expect(rule.level).toBe('error');
        expect(rule.pattern).toBe('');
        expect(rule.threshold).toBe(1);
        expect(rule.window_ms).toBe(60000);
        expect(rule.enabled).toBe(true);
        expect(rule.id).toBeDefined();
      });

      it('should create with custom values', () => {
        const rule = manager.createAlertRule({
          name: 'Custom Rule',
          level: 'warn',
          pattern: 'error',
          threshold: 5,
          window_ms: 300000,
          source: 'api'
        });
        expect(rule.level).toBe('warn');
        expect(rule.pattern).toBe('error');
        expect(rule.threshold).toBe(5);
        expect(rule.window_ms).toBe(300000);
        expect(rule.source).toBe('api');
      });
    });

    describe('getAlertRules', () => {
      it('should return all alert rules', () => {
        manager.createAlertRule({ name: 'Rule 1' });
        manager.createAlertRule({ name: 'Rule 2' });
        const rules = manager.getAlertRules();
        expect(rules).toHaveLength(2);
      });
    });

    describe('updateAlertRule', () => {
      it('should update existing rule', () => {
        const rule = manager.createAlertRule({ name: 'Original' });
        const updated = manager.updateAlertRule(rule.id, { name: 'Updated', level: 'warn' });
        expect(updated.name).toBe('Updated');
        expect(updated.level).toBe('warn');
      });

      it('should return null for non-existent rule', () => {
        const result = manager.updateAlertRule('non-existent-id', { name: 'Test' });
        expect(result).toBeNull();
      });
    });

    describe('deleteAlertRule', () => {
      it('should delete existing rule', () => {
        const rule = manager.createAlertRule({ name: 'To Delete' });
        expect(manager.getAlertRules()).toHaveLength(1);
        const result = manager.deleteAlertRule(rule.id);
        expect(result).toBe(true);
        expect(manager.getAlertRules()).toHaveLength(0);
      });

      it('should return false for non-existent rule', () => {
        const result = manager.deleteAlertRule('non-existent-id');
        expect(result).toBe(false);
      });
    });
  });

  describe('checkAlerts', () => {
    it('should trigger alert for matching log', () => {
      const rule = manager.createAlertRule({
        name: 'Error Alert',
        level: 'error',
        pattern: '',
        threshold: 1
      });
      const cb = jest.fn();
      manager.onAlertTriggered = cb;

      manager.checkAlerts({
        id: 'log-1',
        level: 'error',
        message: 'Something went wrong',
        source: 'test',
        timestamp: new Date().toISOString()
      });

      expect(cb).toHaveBeenCalledWith(expect.objectContaining({
        name: 'Error Alert',
        severity: 'error'
      }));
    });

    it('should match pattern in message', () => {
      const rule = manager.createAlertRule({
        name: 'Auth Alert',
        level: 'error',
        pattern: 'failed',
        threshold: 1
      });
      const cb = jest.fn();
      manager.onAlertTriggered = cb;

      manager.checkAlerts({
        id: 'log-1',
        level: 'error',
        message: 'Authentication failed for user',
        source: 'test',
        timestamp: new Date().toISOString()
      });

      expect(cb).toHaveBeenCalled();
    });

    it('should filter by source if specified', () => {
      const rule = manager.createAlertRule({
        name: 'API Error',
        level: 'error',
        source: 'api',
        threshold: 1
      });
      const cb = jest.fn();
      manager.onAlertTriggered = cb;

      // Log from different source
      manager.checkAlerts({
        id: 'log-1',
        level: 'error',
        message: 'Error occurred',
        source: 'web',
        timestamp: new Date().toISOString()
      });

      expect(cb).not.toHaveBeenCalled();
    });

    it('should not trigger disabled rules', () => {
      const rule = manager.createAlertRule({
        name: 'Disabled Rule',
        level: 'error',
        enabled: false,
        threshold: 1
      });
      const cb = jest.fn();
      manager.onAlertTriggered = cb;

      manager.checkAlerts({
        id: 'log-1',
        level: 'error',
        message: 'Error',
        source: 'test',
        timestamp: new Date().toISOString()
      });

      expect(cb).not.toHaveBeenCalled();
    });
  });

  describe('Saved Filters', () => {
    describe('saveFilter', () => {
      it('should save filter', () => {
        const filter = manager.saveFilter({
          name: 'My Filter',
          query: { level: 'error' }
        });
        expect(filter.name).toBe('My Filter');
        expect(filter.id).toBe('test-uuid-1');
      });

      it('should update existing filter', () => {
        const filter = manager.saveFilter({ name: 'Original', query: {} });
        const updated = manager.saveFilter({ id: filter.id, name: 'Updated', query: { level: 'warn' } });
        expect(updated.name).toBe('Updated');
        expect(manager.filters).toHaveLength(1);
      });
    });

    describe('getSavedFilters', () => {
      it('should return all saved filters', () => {
        manager.saveFilter({ name: 'Filter 1', query: {} });
        manager.saveFilter({ name: 'Filter 2', query: {} });
        const filters = manager.getSavedFilters();
        expect(filters).toHaveLength(2);
      });
    });

    describe('deleteFilter', () => {
      it('should delete existing filter', () => {
        const filter = manager.saveFilter({ name: 'To Delete', query: {} });
        const result = manager.deleteFilter(filter.id);
        expect(result).toBe(true);
        expect(manager.getSavedFilters()).toHaveLength(0);
      });

      it('should return false for non-existent filter', () => {
        const result = manager.deleteFilter('non-existent');
        expect(result).toBe(false);
      });
    });
  });

  describe('Retention', () => {
    describe('getRetentionConfig', () => {
      it('should return retention config', () => {
        const config = manager.getRetentionConfig();
        expect(config.retention_days).toBeDefined();
        expect(config.backend).toBeDefined();
        expect(config.backend_type).toBeDefined();
      });
    });

    describe('updateRetentionConfig', () => {
      it('should update retention days', () => {
        const config = manager.updateRetentionConfig({ retention_days: 30 });
        expect(config.retention_days).toBe(30);
      });

      it('should clamp negative retention days to 1', () => {
        const config = manager.updateRetentionConfig({ retention_days: -5 });
        expect(config.retention_days).toBe(1);
      });

      it('should cap retention days at 3650', () => {
        const config = manager.updateRetentionConfig({ retention_days: 5000 });
        expect(config.retention_days).toBe(3650);
      });
    });

    describe('applyRetention', () => {
      it('should delete logs older than retention period', async () => {
        const oldDate = new Date();
        oldDate.setDate(oldDate.getDate() - 100);
        manager.logs = [
          { id: 'old', timestamp: oldDate.toISOString(), level: 'info', message: 'old', source: 's', resource: null, metadata: {}, tags: [] },
          { id: 'new', timestamp: new Date().toISOString(), level: 'info', message: 'new', source: 's', resource: null, metadata: {}, tags: [] }
        ];

        manager.retentionDays = 30;
        const result = await manager.applyRetention();
        expect(result.deleted).toBe(1);
        expect(manager.logs).toHaveLength(1);
        expect(manager.logs[0].id).toBe('new');
      });
    });
  });

  describe('getBackendHealth', () => {
    it('should return backend health', async () => {
      const health = await manager.getBackendHealth();
      expect(health.healthy).toBe(true);
      expect(health.backend).toBeDefined();
    });
  });

  describe('generateSampleLogs', () => {
    it('should generate sample logs', () => {
      const result = manager.generateSampleLogs(10);
      expect(result.generated).toBe(10);
    });
  });
});
