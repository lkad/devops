const DeviceManager = require('../devices/device_manager');
const path = require('path');
const fs = require('fs');
const { v4: uuidv4 } = require('uuid');

describe('DeviceManager', () => {
  const testConfigDir = '/tmp/devops-test-device-manager';
  let deviceManager;

  beforeAll(() => {
    // Create test config directory
    if (!fs.existsSync(testConfigDir)) {
      fs.mkdirSync(testConfigDir, { recursive: true });
    }
  });

  beforeEach(() => {
    // Create fresh DeviceManager for each test
    deviceManager = new DeviceManager(testConfigDir);
  });

  afterEach(() => {
    // Clean up test devices file
    const deviceFile = path.join(testConfigDir, 'devices.json');
    if (fs.existsSync(deviceFile)) {
      fs.unlinkSync(deviceFile);
    }
  });

  afterAll(() => {
    // Clean up test config directory
    if (fs.existsSync(testConfigDir)) {
      fs.rmSync(testConfigDir, { recursive: true, force: true });
    }
  });

  describe('registerDevice', () => {
    it('should register a new device with required fields', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-container-01'
      });

      expect(device).toBeDefined();
      expect(device.id).toBeDefined();
      expect(device.type).toBe('container');
      expect(device.name).toBe('test-container-01');
      expect(device.status).toBe('pending');
    });

    it('should register a device with custom id', () => {
      const customId = 'custom-device-001';
      const device = deviceManager.registerDevice({
        id: customId,
        type: 'physical_host',
        name: 'test-physical-host'
      });

      expect(device.id).toBe(customId);
    });

    it('should register a device with labels', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-with-labels',
        labels: [
          { env: 'development' },
          { device_type: 'web' }
        ]
      });

      expect(device.labels).toHaveLength(2);
      expect(device.labels).toContainEqual({ env: 'development' });
      expect(device.labels).toContainEqual({ device_type: 'web' });
    });

    it('should register a device with business_unit and compute_cluster', () => {
      const device = deviceManager.registerDevice({
        type: 'cloud_instance',
        name: 'test-cloud-instance',
        business_unit: 'engineering',
        compute_cluster: 'prod-cluster-1'
      });

      expect(device.business_unit).toBe('engineering');
      expect(device.compute_cluster).toBe('prod-cluster-1');
    });
  });

  describe('getDevice', () => {
    it('should retrieve a registered device by id', () => {
      const registered = deviceManager.registerDevice({
        type: 'container',
        name: 'test-get-device'
      });

      const retrieved = deviceManager.getDevice(registered.id);

      expect(retrieved).toBeDefined();
      expect(retrieved.id).toBe(registered.id);
      expect(retrieved.name).toBe('test-get-device');
    });

    it('should return undefined for non-existent device', () => {
      const result = deviceManager.getDevice('non-existent-id');
      expect(result).toBeUndefined();
    });
  });

  describe('getDevicesByTags', () => {
    beforeEach(() => {
      // Register multiple devices with different tags
      deviceManager.registerDevice({
        type: 'container',
        name: 'web-server-01',
        labels: [
          { env: 'development' },
          { device_type: 'web' }
        ],
        business_unit: 'frontend'
      });

      deviceManager.registerDevice({
        type: 'container',
        name: 'web-server-02',
        labels: [
          { env: 'development' },
          { device_type: 'web' }
        ],
        business_unit: 'frontend'
      });

      deviceManager.registerDevice({
        type: 'container',
        name: 'app-server-01',
        labels: [
          { env: 'development' },
          { device_type: 'app' }
        ],
        business_unit: 'backend'
      });

      deviceManager.registerDevice({
        type: 'network_device',
        name: 'switch-01',
        labels: [
          { env: 'production' },
          { device_type: 'network' }
        ],
        business_unit: 'infrastructure'
      });
    });

    it('should filter devices by tag', () => {
      const webDevices = deviceManager.getDevicesByTags(['env:development', 'device_type:web']);

      expect(webDevices).toHaveLength(2);
      expect(webDevices.map(d => d.name).sort()).toEqual(['web-server-01', 'web-server-02']);
    });

    it('should return empty array when no devices match', () => {
      const result = deviceManager.getDevicesByTags(['env:staging', 'device_type:database']);
      expect(result).toHaveLength(0);
    });
  });

  describe('updateConfig', () => {
    it('should update device configuration', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-update-config'
      });

      const result = deviceManager.updateConfig(device.id, {
        template: 'web-server-template',
        version: '1.1'
      });

      expect(result.success).toBe(true);
      expect(result.device.config.template).toBe('web-server-template');
      expect(result.device.config.version).toBe('1.1');
    });

    it('should return failure for non-existent device', () => {
      const result = deviceManager.updateConfig('non-existent-id', {
        template: 'test'
      });

      expect(result.success).toBe(false);
      expect(result.error).toBe('Device not found');
    });

    it('should preserve existing config and merge updates', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-merge-config'
      });

      deviceManager.updateConfig(device.id, {
        template: 'initial-template',
        version: '1.0'
      });

      const updated = deviceManager.updateConfig(device.id, {
        version: '2.0'
      });

      expect(updated.device.config.template).toBe('initial-template');
      expect(updated.device.config.version).toBe('2.0');
    });
  });

  describe('removeDevice', () => {
    it('should remove an existing device', () => {
      const device = deviceManager.registerDevice({
        type: 'container',
        name: 'test-remove-device'
      });

      const result = deviceManager.removeDevice(device.id);

      expect(result.success).toBe(true);
      expect(deviceManager.getDevice(device.id)).toBeUndefined();
    });

    it('should return failure for non-existent device', () => {
      const result = deviceManager.removeDevice('non-existent-id');

      expect(result.success).toBe(false);
      expect(result.error).toBe('Device not found');
    });
  });

  describe('getAllDevices', () => {
    it('should return all registered devices', () => {
      deviceManager.registerDevice({
        type: 'container',
        name: 'device-1'
      });

      deviceManager.registerDevice({
        type: 'network_device',
        name: 'device-2'
      });

      const allDevices = deviceManager.getAllDevices();

      expect(allDevices).toHaveLength(2);
    });

    it('should return empty array when no devices registered', () => {
      const allDevices = deviceManager.getAllDevices();
      expect(allDevices).toHaveLength(0);
    });
  });

  describe('persistence', () => {
    it('should persist devices to file', () => {
      deviceManager.registerDevice({
        type: 'container',
        name: 'persistent-device'
      });

      // Create new instance - should load existing devices
      const newManager = new DeviceManager(testConfigDir);
      const devices = newManager.getAllDevices();

      expect(devices).toHaveLength(1);
      expect(devices[0].name).toBe('persistent-device');
    });
  });
});
