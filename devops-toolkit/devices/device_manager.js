// devices/device_manager.js - 设备管理器

const path = require('path');
const fs = require('fs');
const { v4: uuidv4 } = require('uuid');

class DeviceManager {
  constructor(configDir) {
    this.devices = new Map();
    this.configDir = configDir || path.join(__dirname, '../config/devices');
    
    // 设备类型
    this.deviceTypes = {
      physical_host: ['SSH', 'SCP', 'WinRM'],
      container: ['Docker', 'Kubernetes'],
      network_device: ['SNMP', 'NETCONF'],
      cloud_instance: ['API'],
      iot_device: ['MQTT', 'HTTP']
    };
    
    // 加载已有设备
    this.loadDevices();
  }
  
  // 注册设备
  registerDevice(options) {
    const device = {
      id: options.id || uuidv4(),
      type: options.type,
      name: options.name || `device-${device.type}-${Date.now()}`,
      
      // 标签
      labels: options.labels || [],
      
      // 业务组映射
      business_unit: options.business_unit || null,
      
      // 关联关系
      compute_cluster: options.compute_cluster || null,
      
      // 状态
      status: 'pending',
      
      // 配置
      config: {
        template: null,
        version: '1.0',
        sync_at: null
      }
    };
    
    this.devices.set(device.id, device);
    this.saveDevice(device.id);
    
    console.log(`[DeviceManager] Device registered: ${device.id} (${device.type})`);
    
    return device;
  }
  
  // 获取设备
  getDevice(id) {
    return this.devices.get(id) || this.getDeviceByTag(id);
  }
  
  // 批量获取设备
  getDevicesByTags(tags) {
    const result = [];
    
    for (const device of this.devices.values()) {
      let matched = false;
      
      for (const tag of tags) {
        for (const label of device.labels) {
          if (label[tag.split(':')[0]] === tag.split(':')[1]) {
            matched = true;
            break;
          }
        }
      }
      
      if (matched) {
        result.push(device);
      }
    }
    
    return result;
  }
  
  // 更新设备配置
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
    
    this.saveDevice(deviceId);
    
    return {
      success: true,
      device: device
    };
  }
  
  // 批量操作
  bulkOperation(operation, devices) {
    const results = [];
    
    for (const devId of devices) {
      const device = this.devices.get(devId);
      if (device) {
        const result = await operation(device);
        results.push({ device_id: devId, ...result });
      }
    }
    
    return results;
  }
  
  // 加载设备
  loadDevices() {
    const deviceFile = path.join(this.configDir, 'devices.json');
    
    try {
      const data = JSON.parse(fs.readFileSync(deviceFile, 'utf8'));
      
      for (const device of data.devices) {
        this.devices.set(device.id, device);
      }
      
      console.log(`[DeviceManager] Loaded ${data.devices.length} devices`);
    } catch (err) {
      this.saveDevices();
    }
  }
  
  // 保存设备
  saveDevice(id) {
    this.saveDevices();
  }
  
  // 保存所有设备
  saveDevices() {
    const deviceFile = path.join(this.configDir, 'devices.json');
    fs.writeFileSync(deviceFile, JSON.stringify({
      devices: Array.from(this.devices.values())
    }, null, 2));
  }
  
  // 移除设备
  removeDevice(id) {
    if (!this.devices.has(id)) {
      return { success: false, error: 'Device not found' };
    }
    
    this.devices.delete(id);
    this.saveDevices();
    
    return { success: true };
  }
  
  // 获取所有设备
  getAllDevices() {
    return Array.from(this.devices.values());
  }
}

module.exports = DeviceManager;
