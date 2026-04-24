/**
 * Tests for Device State Machine
 */

const DeviceManager = require('../devices/device_manager');
const path = require('path');
const fs = require('fs');

describe('DeviceManager State Machine', () => {
  const testConfigDir = '/tmp/devops-test-state-machine';
  let deviceManager;

  beforeAll(() => {
    if (!fs.existsSync(testConfigDir)) {
      fs.mkdirSync(testConfigDir, { recursive: true });
    }
  });

  beforeEach(() => {
    deviceManager = new DeviceManager(testConfigDir);
  });

  afterEach(() => {
    const deviceFile = path.join(testConfigDir, 'devices.json');
    if (fs.existsSync(deviceFile)) {
      fs.unlinkSync(deviceFile);
    }
    const groupsFile = path.join(testConfigDir, 'groups.json');
    if (fs.existsSync(groupsFile)) {
      fs.unlinkSync(groupsFile);
    }
  });

  afterAll(() => {
    if (fs.existsSync(testConfigDir)) {
      fs.rmSync(testConfigDir, { recursive: true, force: true });
    }
  });

  // Helper to set up an active device
  const setupActiveDevice = () => {
    const device = deviceManager.registerDevice({ type: 'container' });
    deviceManager.authenticateDevice(device.id, { token: 'valid-token' });
    deviceManager.completeRegistration(device.id, {});
    deviceManager.activateDevice(device.id);
    return device;
  };

  describe('Initial State', () => {
    it('should register device in PENDING state', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-container'
      });

      expect(device.status).toBe('pending');
      expect(device.state_history).toHaveLength(1);
      expect(device.state_history[0].from).toBeNull();
      expect(device.state_history[0].to).toBe('pending');
    });
  });

  describe('State Transitions', () => {
    it('should transition PENDING -> AUTHENTICATED', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      const result = deviceManager.authenticateDevice(device.id, { token: 'valid-token' });

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('authenticated');
    });

    it('should transition AUTHENTICATED -> REGISTERED', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.authenticateDevice(device.id, { token: 'valid-token' });
      const result = deviceManager.completeRegistration(device.id, {});

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('registered');
    });

    it('should transition REGISTERED -> ACTIVE', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.authenticateDevice(device.id, { token: 'valid-token' });
      deviceManager.completeRegistration(device.id, {});
      const result = deviceManager.activateDevice(device.id);

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('active');
    });

    it('should transition ACTIVE -> MAINTENANCE', () => {
      const device = setupActiveDevice();

      const result = deviceManager.enterMaintenance(device.id, 'Scheduled maintenance');

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('maintenance');
      expect(result.device.maintenance_reason).toBe('Scheduled maintenance');
    });

    it('should transition MAINTENANCE -> ACTIVE', () => {
      const device = setupActiveDevice();
      deviceManager.enterMaintenance(device.id);

      const result = deviceManager.exitMaintenance(device.id);

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('active');
    });

    it('should transition ACTIVE -> SUSPENDED', () => {
      const device = setupActiveDevice();

      const result = deviceManager.suspendDevice(device.id, 'Billing issue');

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('suspended');
    });

    it('should transition to RETIRE', () => {
      const device = setupActiveDevice();

      const result = deviceManager.retireDevice(device.id);

      expect(result.success).toBe(true);
      expect(result.device.status).toBe('retire');
    });

    it('should reject invalid state transitions', () => {
      const device = deviceManager.registerDevice({ type: 'container' });

      // Cannot go directly from PENDING to ACTIVE
      const result = deviceManager.activateDevice(device.id);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Cannot activate from state');
    });

    it('should record state history', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.authenticateDevice(device.id, { token: 'valid-token' });
      deviceManager.completeRegistration(device.id, {});
      deviceManager.activateDevice(device.id);

      expect(device.state_history).toHaveLength(4);
      expect(device.state_history.map(h => h.to)).toEqual([
        'pending',
        'authenticated',
        'registered',
        'active'
      ]);
    });
  });

  describe('Invalid Transitions', () => {
    it('should reject transition from RETIRE state', () => {
      const device = setupActiveDevice();
      deviceManager.retireDevice(device.id);

      const result = deviceManager.activateDevice(device.id);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Cannot activate from state');
    });

    it('should reject authenticate from non-PENDING state', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.authenticateDevice(device.id, { token: 'valid-token' });

      const result = deviceManager.authenticateDevice(device.id, { token: 'valid-token' });

      expect(result.success).toBe(false);
    });
  });

  describe('Device Queries', () => {
    it('should get devices by state', () => {
      const d1 = deviceManager.registerDevice({ type: 'container', name: 'd1' });
      const d2 = deviceManager.registerDevice({ type: 'container', name: 'd2' });
      deviceManager.authenticateDevice(d1.id, { token: 'token' });

      const pending = deviceManager.getDevicesByState('pending');
      const authenticated = deviceManager.getDevicesByState('authenticated');

      expect(pending.map(d => d.id)).toContain(d2.id);
      expect(authenticated.map(d => d.id)).toContain(d1.id);
    });

    it('should get state history', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.authenticateDevice(device.id, { token: 'token' });

      const history = deviceManager.getStateHistory(device.id);

      expect(history).toHaveLength(2);
      expect(history[0].to).toBe('pending');
      expect(history[1].to).toBe('authenticated');
    });
  });

  describe('Device Hierarchy', () => {
    it('should create parent-child relationships', () => {
      const parent = deviceManager.registerDevice({ type: 'physical_host', name: 'parent' });
      const child = deviceManager.registerDevice({
        type: 'container',
        name: 'child',
        parent_id: parent.id
      });

      const retrievedChild = deviceManager.getDevice(child.id);
      expect(retrievedChild.parent_id).toBe(parent.id);

      const children = deviceManager.getDeviceChildren(parent.id);
      expect(children.map(c => c.id)).toContain(child.id);
    });

    it('should get full device hierarchy', () => {
      const parent = deviceManager.registerDevice({ type: 'physical_host', name: 'parent' });
      const child = deviceManager.registerDevice({
        type: 'container',
        name: 'child',
        parent_id: parent.id
      });

      const hierarchy = deviceManager.getDeviceHierarchy(parent.id);

      expect(hierarchy.id).toBe(parent.id);
      expect(hierarchy.children).toHaveLength(1);
      expect(hierarchy.children[0].id).toBe(child.id);
    });
  });

  describe('Device Groups', () => {
    it('should create and retrieve groups', () => {
      const group = deviceManager.createGroup('web-servers', {
        description: 'Web server devices',
        tags: ['env:prod']
      });

      const retrieved = deviceManager.getGroup('web-servers');
      expect(retrieved.name).toBe('web-servers');
      expect(retrieved.description).toBe('Web server devices');
    });

    it('should add device to group', () => {
      const device = deviceManager.registerDevice({ type: 'container' });
      deviceManager.createGroup('test-group');

      const result = deviceManager.addDeviceToGroup('test-group', device.id);

      expect(result.success).toBe(true);
      expect(result.group.device_ids).toContain(device.id);
    });

    it('should get devices in group', () => {
      const d1 = deviceManager.registerDevice({ type: 'container', name: 'd1' });
      const d2 = deviceManager.registerDevice({ type: 'container', name: 'd2' });
      deviceManager.createGroup('test-group');
      deviceManager.addDeviceToGroup('test-group', d1.id);
      deviceManager.addDeviceToGroup('test-group', d2.id);

      const devices = deviceManager.getDevicesInGroup('test-group');

      expect(devices).toHaveLength(2);
    });
  });

  describe('Bulk Operations', () => {
    it('should bulk update by tags', () => {
      const d1 = deviceManager.registerDevice({
        type: 'container',
        name: 'd1',
        labels: [{ env: 'prod' }]
      });
      const d2 = deviceManager.registerDevice({
        type: 'container',
        name: 'd2',
        labels: [{ env: 'prod' }]
      });
      deviceManager.registerDevice({
        type: 'container',
        name: 'd3',
        labels: [{ env: 'dev' }]
      });

      const results = deviceManager.bulkUpdateByTags(['env:prod'], { version: '2.0' });

      expect(results).toHaveLength(2);
      expect(results.every(r => r.success)).toBe(true);
    });
  });
});
