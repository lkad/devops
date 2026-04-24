/**
 * Tests for DevTools Agent
 * Covers: agent initialization, connection, configuration
 */

const DevToolsAgent = require('../devices/agent');

describe('DevToolsAgent', () => {
  let agent;

  beforeEach(() => {
    agent = new DevToolsAgent({
      deviceType: 'container'
    });
  });

  describe('Initialization', () => {
    it('should create agent', () => {
      expect(agent).toBeDefined();
      expect(agent.id).toBeDefined();
    });

    it('should set device type', () => {
      const typedAgent = new DevToolsAgent({ deviceType: 'sensor' });
      expect(typedAgent.deviceType).toBe('sensor');
    });

    it('should have initial state', () => {
      expect(agent.state).toBeDefined();
      expect(agent.state.health).toBe('healthy');
    });

    it('should have empty connected state', () => {
      expect(agent.connected).toBe(false);
    });
  });

  describe('Configuration', () => {
    it('should have applyConfiguration method', () => {
      expect(typeof agent.applyConfiguration).toBe('function');
    });

    it('should apply configuration to device', () => {
      const config = { setting: 'test' };
      expect(() => agent.applyToDevice(config)).not.toThrow();
    });
  });

  describe('Connection', () => {
    it('should have connect method', () => {
      expect(typeof agent.connect).toBe('function');
    });

    it('should have close method', () => {
      expect(typeof agent.close).toBe('function');
    });

    it('should handle server messages', () => {
      expect(typeof agent.handleServerMessage).toBe('function');
    });
  });

  describe('Status Reporting', () => {
    it('should send heartbeat', () => {
      expect(typeof agent.sendHeartbeat).toBe('function');
      expect(() => agent.sendHeartbeat()).not.toThrow();
    });

    it('should send status', () => {
      expect(typeof agent.sendStatus).toBe('function');
      expect(() => agent.sendStatus({})).not.toThrow();
    });

    it('should get state', () => {
      expect(typeof agent.getState).toBe('function');
      const state = agent.getState();
      expect(state).toBeDefined();
    });
  });

  describe('Labels', () => {
    it('should get labels', () => {
      expect(typeof agent.getLabels).toBe('function');
    });

    it('should get initial labels', () => {
      expect(typeof agent.getInitialLabels).toBe('function');
      const labels = agent.getInitialLabels();
      expect(Array.isArray(labels)).toBe(true);
    });
  });

  describe('Event Handling', () => {
    it('should extend EventEmitter', () => {
      expect(typeof agent.on).toBe('function');
      expect(typeof agent.emit).toBe('function');
    });
  });

  describe('Network Configuration', () => {
    it('should apply network config', () => {
      const config = { network: { key: 'value' } };
      expect(() => agent.applyNetworkConfig(config)).not.toThrow();
    });
  });
});
