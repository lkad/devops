/**
 * Device Manager
 * Manages devices with state machine, configuration templates, and group hierarchy
 */

const path = require('path');
const fs = require('fs');
const { v4: uuidv4 } = require('uuid');

// Device States
const DeviceState = {
  PENDING: 'pending',
  AUTHENTICATED: 'authenticated',
  REGISTERED: 'registered',
  ACTIVE: 'active',
  MAINTENANCE: 'maintenance',
  SUSPENDED: 'suspended',
  RETIRE: 'retire'
};

// Valid state transitions
const STATE_TRANSITIONS = {
  [DeviceState.PENDING]: [DeviceState.AUTHENTICATED],
  [DeviceState.AUTHENTICATED]: [DeviceState.REGISTERED, DeviceState.PENDING],
  [DeviceState.REGISTERED]: [DeviceState.ACTIVE, DeviceState.SUSPENDED],
  [DeviceState.ACTIVE]: [DeviceState.MAINTENANCE, DeviceState.SUSPENDED, DeviceState.RETIRE],
  [DeviceState.MAINTENANCE]: [DeviceState.ACTIVE, DeviceState.SUSPENDED],
  [DeviceState.SUSPENDED]: [DeviceState.REGISTERED, DeviceState.RETIRE],
  [DeviceState.RETIRE]: []
};

class DeviceManager {
  constructor(configDir) {
    this.devices = new Map();
    this.configDir = configDir || path.join(__dirname, '../config/devices');
    this.groups = new Map();
    this.templates = new Map();

    // Device type definitions
    this.deviceTypes = {
      physical_host: {
        protocols: ['SSH', 'SCP', 'WinRM'],
        discovery: 'active_registration'
      },
      container: {
        protocols: ['Docker', 'Kubernetes'],
        discovery: 'auto_discovery'
      },
      network_device: {
        protocols: ['SNMP', 'NETCONF'],
        discovery: 'pull_registration'
      },
      load_balancer: {
        protocols: ['HTTP', 'REST_API'],
        discovery: 'config_import'
      },
      cloud_instance: {
        protocols: ['SSH', 'Custom'],
        discovery: 'cloud_api'
      },
      iot_device: {
        protocols: ['MQTT', 'HTTP'],
        discovery: 'ota_registration'
      }
    };

    // Load existing data
    this.load();
  }

  // ============ State Machine ============

  /**
   * Transition device to a new state
   */
  transitionState(deviceId, newState) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    const currentState = device.status;
    const validTransitions = STATE_TRANSITIONS[currentState] || [];

    if (!validTransitions.includes(newState)) {
      return {
        success: false,
        error: `Invalid state transition from '${currentState}' to '${newState}'`,
        validTransitions
      };
    }

    const oldState = device.status;
    device.status = newState;
    device.state_history = device.state_history || [];
    device.state_history.push({
      from: oldState,
      to: newState,
      timestamp: new Date().toISOString()
    });

    this.saveDevice(deviceId);
    return { success: true, device };
  }

  /**
   * Get devices by state
   */
  getDevicesByState(state) {
    const result = [];
    for (const device of this.devices.values()) {
      if (device.status === state) {
        result.push(device);
      }
    }
    return result;
  }

  /**
   * Get device state history
   */
  getStateHistory(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) return null;
    return device.state_history || [];
  }

  // ============ Device Registration ============

  /**
   * Register a new device
   */
  registerDevice(options) {
    const id = options.id || uuidv4();

    // Validate device type
    if (options.type && !this.deviceTypes[options.type]) {
      throw new Error(`Invalid device type: ${options.type}`);
    }

    const device = {
      id,
      type: options.type || 'physical_host',
      name: options.name || `device-${options.type}-${Date.now()}`,

      // Labels (for filtering and permissions)
      labels: options.labels || [],

      // Business unit mapping
      business_unit: options.business_unit || null,

      // Cluster relationship
      compute_cluster: options.compute_cluster || null,

      // Parent-child relationships
      parent_id: options.parent_id || null,
      child_ids: [],

      // State machine
      status: DeviceState.PENDING,
      state_history: [{
        from: null,
        to: DeviceState.PENDING,
        timestamp: new Date().toISOString()
      }],

      // Configuration
      config: {
        template: options.template || null,
        version: '1.0',
        sync_at: null,
        variables: options.variables || {}
      },

      // Metadata
      metadata: options.metadata || {},
      registered_at: new Date().toISOString(),
      last_seen: null,
      last_config_sync: null
    };

    this.devices.set(id, device);

    // Add to parent's children if parent specified
    if (device.parent_id) {
      const parent = this.devices.get(device.parent_id);
      if (parent) {
        parent.child_ids = parent.child_ids || [];
        parent.child_ids.push(id);
      }
    }

    this.saveDevice(id);
    console.log(`[DeviceManager] Device registered: ${id} (${device.type}) - State: ${device.status}`);

    return device;
  }

  /**
   * Authenticate a device
   */
  authenticateDevice(deviceId, credentials) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    if (device.status !== DeviceState.PENDING) {
      return { success: false, error: `Cannot authenticate from state '${device.status}'` };
    }

    // Validate credentials based on device type
    const authResult = this.validateCredentials(device.type, credentials);
    if (!authResult.success) {
      return authResult;
    }

    return this.transitionState(deviceId, DeviceState.AUTHENTICATED);
  }

  /**
   * Validate device credentials
   */
  validateCredentials(deviceType, credentials) {
    // Simplified credential validation
    // In production, implement protocol-specific validation
    if (!credentials || !credentials.token) {
      return { success: false, error: 'Credentials required' };
    }

    return { success: true };
  }

  /**
   * Complete device registration
   */
  completeRegistration(deviceId, registrationData) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    if (device.status !== DeviceState.AUTHENTICATED) {
      return { success: false, error: `Cannot register from state '${device.status}'` };
    }

    // Apply registration data
    if (registrationData.config) {
      device.config = { ...device.config, ...registrationData.config };
    }
    if (registrationData.labels) {
      device.labels = registrationData.labels;
    }
    if (registrationData.metadata) {
      device.metadata = { ...device.metadata, ...registrationData.metadata };
    }

    const result = this.transitionState(deviceId, DeviceState.REGISTERED);
    if (result.success) {
      device.last_seen = new Date().toISOString();
      this.saveDevice(deviceId);
    }
    return result;
  }

  /**
   * Activate a device
   */
  activateDevice(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    if (device.status !== DeviceState.REGISTERED) {
      return { success: false, error: `Cannot activate from state '${device.status}'` };
    }

    device.last_seen = new Date().toISOString();
    return this.transitionState(deviceId, DeviceState.ACTIVE);
  }

  /**
   * Put device into maintenance mode
   */
  enterMaintenance(deviceId, reason = '') {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    if (device.status !== DeviceState.ACTIVE) {
      return { success: false, error: `Cannot enter maintenance from state '${device.status}'` };
    }

    device.maintenance_reason = reason;
    device.maintenance_started = new Date().toISOString();
    return this.transitionState(deviceId, DeviceState.MAINTENANCE);
  }

  /**
   * Exit maintenance mode
   */
  exitMaintenance(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    if (device.status !== DeviceState.MAINTENANCE) {
      return { success: false, error: `Not in maintenance mode` };
    }

    delete device.maintenance_reason;
    delete device.maintenance_started;
    return this.transitionState(deviceId, DeviceState.ACTIVE);
  }

  /**
   * Suspend a device
   */
  suspendDevice(deviceId, reason = '') {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    device.suspend_reason = reason;
    return this.transitionState(deviceId, DeviceState.SUSPENDED);
  }

  /**
   * Retire a device
   */
  retireDevice(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    device.retired_at = new Date().toISOString();
    const result = this.transitionState(deviceId, DeviceState.RETIRE);
    if (result.success) {
      // Remove from parent's children
      if (device.parent_id) {
        const parent = this.devices.get(device.parent_id);
        if (parent && parent.child_ids) {
          parent.child_ids = parent.child_ids.filter(id => id !== deviceId);
        }
      }
    }
    return result;
  }

  // ============ Device Queries ============

  /**
   * Get device by ID
   */
  getDevice(id) {
    return this.devices.get(id);
  }

  /**
   * Get all devices
   */
  getAllDevices() {
    return Array.from(this.devices.values());
  }

  /**
   * Get devices by tags (AND logic)
   */
  getDevicesByTags(tags) {
    const result = [];

    for (const device of this.devices.values()) {
      let matchedAll = true;

      for (const tag of tags) {
        const [key, value] = tag.split(':');
        const labelMatches = device.labels.some(label => {
          if (typeof label === 'object') return label[key] === value;
          return false;
        });
        if (!labelMatches) {
          matchedAll = false;
          break;
        }
      }

      if (matchedAll) {
        result.push(device);
      }
    }

    return result;
  }

  /**
   * Get device children
   */
  getDeviceChildren(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) return [];
    return (device.child_ids || [])
      .map(childId => this.devices.get(childId))
      .filter(Boolean);
  }

  /**
   * Get device hierarchy (device + all descendants)
   */
  getDeviceHierarchy(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) return null;

    const hierarchy = { ...device };
    if (device.child_ids && device.child_ids.length > 0) {
      hierarchy.children = device.child_ids
        .map(childId => this.getDeviceHierarchy(childId))
        .filter(Boolean);
    }

    return hierarchy;
  }

  // ============ Configuration Management ============

  /**
   * Update device configuration
   */
  updateConfig(deviceId, config) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    device.config = {
      ...device.config,
      ...config,
      last_updated: new Date().toISOString()
    };

    device.last_config_sync = new Date().toISOString();
    this.saveDevice(deviceId);

    return { success: true, device };
  }

  /**
   * Apply configuration template to device
   */
  applyTemplate(deviceId, templateName, variables = {}) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    const template = this.templates.get(templateName);
    if (!template) {
      return { success: false, error: `Template '${templateName}' not found` };
    }

    // Merge template with device-specific variables
    const mergedConfig = this.mergeTemplate(template, variables);

    return this.updateConfig(deviceId, {
      template: templateName,
      variables,
      version: this.incrementVersion(device.config.version)
    });
  }

  /**
   * Merge template with variables
   */
  mergeTemplate(template, variables) {
    // Simple variable substitution
    // In production, use Jinja2 or similar
    let result = JSON.stringify(template);

    for (const [key, value] of Object.entries(variables)) {
      result = result.replace(new RegExp(`{{${key}}}`, 'g'), String(value));
    }

    return JSON.parse(result);
  }

  /**
   * Increment version string
   */
  incrementVersion(version) {
    const parts = version.split('.');
    parts[2] = parseInt(parts[2] || '0') + 1;
    return parts.join('.');
  }

  // ============ Device Groups ============

  /**
   * Create a device group
   */
  createGroup(name, options = {}) {
    const group = {
      id: options.id || uuidv4(),
      name,
      description: options.description || '',
      parent_id: options.parent_id || null,
      child_ids: [],
      tags: options.tags || [],
      device_ids: [],
      created_at: new Date().toISOString()
    };

    this.groups.set(name, group);
    this.saveGroups();

    // Add to parent group
    if (group.parent_id) {
      const parent = this.groups.get(group.parent_id);
      if (parent) {
        parent.child_ids.push(name);
      }
    }

    return group;
  }

  /**
   * Get group
   */
  getGroup(name) {
    return this.groups.get(name);
  }

  /**
   * Add device to group
   */
  addDeviceToGroup(groupName, deviceId) {
    const group = this.groups.get(groupName);
    if (!group) {
      return { success: false, error: 'Group not found' };
    }

    if (!group.device_ids.includes(deviceId)) {
      group.device_ids.push(deviceId);
      this.saveGroups();
    }

    return { success: true, group };
  }

  /**
   * Get devices in group (including children)
   */
  getDevicesInGroup(groupName) {
    const group = this.groups.get(groupName);
    if (!group) return [];

    const devices = group.device_ids
      .map(id => this.devices.get(id))
      .filter(Boolean);

    // Include children recursively
    for (const childName of group.child_ids || []) {
      const childDevices = this.getDevicesInGroup(childName);
      devices.push(...childDevices);
    }

    return devices;
  }

  // ============ Bulk Operations ============

  /**
   * Bulk update devices by tags
   */
  bulkUpdateByTags(tags, updates) {
    const devices = this.getDevicesByTags(tags);
    const results = [];

    for (const device of devices) {
      const result = this.updateConfig(device.id, updates);
      results.push({ device_id: device.id, ...result });
    }

    return results;
  }

  /**
   * Bulk state transition
   */
  bulkStateTransition(deviceIds, newState) {
    const results = [];

    for (const deviceId of deviceIds) {
      const result = this.transitionState(deviceId, newState);
      results.push({ device_id: deviceId, ...result });
    }

    return results;
  }

  // ============ Persistence ============

  /**
   * Load devices and groups from disk
   */
  load() {
    // Load devices
    const deviceFile = path.join(this.configDir, 'devices.json');
    try {
      if (fs.existsSync(deviceFile)) {
        const data = JSON.parse(fs.readFileSync(deviceFile, 'utf8'));
        for (const device of data.devices || []) {
          this.devices.set(device.id, device);
        }
        console.log(`[DeviceManager] Loaded ${data.devices?.length || 0} devices`);
      }
    } catch (err) {
      console.error('[DeviceManager] Failed to load devices:', err.message);
    }

    // Load groups
    const groupsFile = path.join(this.configDir, 'groups.json');
    try {
      if (fs.existsSync(groupsFile)) {
        const data = JSON.parse(fs.readFileSync(groupsFile, 'utf8'));
        for (const [name, group] of data.groups || []) {
          this.groups.set(name, group);
        }
        console.log(`[DeviceManager] Loaded ${data.groups?.length || 0} groups`);
      }
    } catch (err) {
      console.error('[DeviceManager] Failed to load groups:', err.message);
    }
  }

  /**
   * Save a single device
   */
  saveDevice(id) {
    this.saveDevices();
  }

  /**
   * Save all devices
   */
  saveDevices() {
    const deviceFile = path.join(this.configDir, 'devices.json');
    fs.writeFileSync(deviceFile, JSON.stringify({
      devices: Array.from(this.devices.values())
    }, null, 2));
  }

  /**
   * Save groups
   */
  saveGroups() {
    const groupsFile = path.join(this.configDir, 'groups.json');
    fs.writeFileSync(groupsFile, JSON.stringify({
      groups: Array.from(this.groups.entries())
    }, null, 2));
  }

  /**
   * Remove device
   */
  removeDevice(id) {
    if (!this.devices.has(id)) {
      return { success: false, error: 'Device not found' };
    }

    const device = this.devices.get(id);

    // Remove from parent's children
    if (device.parent_id) {
      const parent = this.devices.get(device.parent_id);
      if (parent && parent.child_ids) {
        parent.child_ids = parent.child_ids.filter(childId => childId !== id);
      }
    }

    // Remove from any groups
    for (const group of this.groups.values()) {
      group.device_ids = group.device_ids.filter(deviceId => deviceId !== id);
    }

    this.devices.delete(id);
    this.saveDevices();
    this.saveGroups();

    return { success: true };
  }

  /**
   * Update device labels
   */
  updateLabels(deviceId, labels) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    device.labels = labels;
    this.saveDevice(deviceId);

    return { success: true, device };
  }

  /**
   * Heartbeat - update last_seen timestamp
   */
  heartbeat(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) {
      return { success: false, error: 'Device not found' };
    }

    device.last_seen = new Date().toISOString();

    // Auto-transition to active if registered
    if (device.status === DeviceState.REGISTERED) {
      this.activateDevice(deviceId);
    }

    return { success: true };
  }
}

// Export enums alongside class
DeviceManager.DeviceState = DeviceState;
DeviceManager.STATE_TRANSITIONS = STATE_TRANSITIONS;

module.exports = DeviceManager;
