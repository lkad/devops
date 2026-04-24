/**
 * Tests for WebSocket Manager
 * Covers: client management, channel subscriptions, broadcasting
 */

describe('WebSocketManager', () => {
  let wsManager;

  beforeEach(() => {
    jest.resetModules();
    wsManager = require('../websocket_manager');
  });

  describe('Client Management', () => {
    it('should initialize', () => {
      expect(wsManager).toBeDefined();
    });

    it('should track client count', () => {
      const count = wsManager.getClientCount();
      expect(typeof count).toBe('number');
    });

    it('should start at 0 clients', () => {
      const count = wsManager.getClientCount();
      expect(count).toBe(0);
    });
  });

  describe('Channel Management', () => {
    it('should get channel stats', () => {
      const stats = wsManager.getChannelStats();
      expect(typeof stats).toBe('object');
    });

    it('should return empty object when no subscriptions', () => {
      const stats = wsManager.getChannelStats();
      expect(Object.keys(stats).length).toBe(0);
    });
  });

  describe('Broadcast', () => {
    it('should broadcast log event', () => {
      const log = {
        id: 'log-1',
        timestamp: new Date().toISOString(),
        level: 'info',
        message: 'Test log'
      };

      expect(() => wsManager.broadcastLog(log)).not.toThrow();
    });

    it('should broadcast metric', () => {
      const metric = {
        name: 'test_metric',
        value: 100,
        labels: {}
      };

      expect(() => wsManager.broadcastMetric(metric)).not.toThrow();
    });

    it('should broadcast device event', () => {
      const event = {
        type: 'device_registered',
        device_id: 'dev-123',
        timestamp: new Date().toISOString()
      };

      expect(() => wsManager.broadcastDeviceEvent(event)).not.toThrow();
    });

    it('should broadcast pipeline update', () => {
      const update = {
        pipeline_id: 'pipe-1',
        status: 'running',
        stage: 'build'
      };

      expect(() => wsManager.broadcastPipelineUpdate(update)).not.toThrow();
    });

    it('should broadcast alert', () => {
      const alert = {
        name: 'Test Alert',
        severity: 'warning',
        message: 'Test'
      };

      expect(() => wsManager.broadcastAlert(alert)).not.toThrow();
    });
  });

  describe('sendToClient', () => {
    it('should handle sending to non-existent client', () => {
      expect(() => wsManager.sendToClient('non-existent-id', { type: 'test' })).not.toThrow();
    });
  });

  describe('handleMessage', () => {
    it('should handle subscribe message', () => {
      const message = { type: 'subscribe', channel: 'logs' };
      expect(() => wsManager.handleMessage('client-1', JSON.stringify(message))).not.toThrow();
    });

    it('should handle unsubscribe message', () => {
      const message = { type: 'unsubscribe', channel: 'logs' };
      expect(() => wsManager.handleMessage('client-1', JSON.stringify(message))).not.toThrow();
    });

    it('should handle ping message', () => {
      const message = { type: 'ping' };
      expect(() => wsManager.handleMessage('client-1', JSON.stringify(message))).not.toThrow();
    });

    it('should handle unknown message type', () => {
      const message = { type: 'unknown_type' };
      expect(() => wsManager.handleMessage('client-1', JSON.stringify(message))).not.toThrow();
    });
  });

  describe('broadcast (general)', () => {
    it('should broadcast to channel', () => {
      expect(() => wsManager.broadcast('test_channel', { data: 'test' })).not.toThrow();
    });

    it('should handle broadcast with complex data', () => {
      const data = {
        nested: { value: 123 },
        array: [1, 2, 3],
        string: 'test'
      };
      expect(() => wsManager.broadcast('test_channel', data)).not.toThrow();
    });
  });
});
