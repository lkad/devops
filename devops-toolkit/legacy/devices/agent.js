// devices/agent.js - DevTools Agent 核心

const EventEmitter = require('events');
const fs = require('fs');
const path = require('path');
const net = require('net');

class DevToolsAgent extends EventEmitter {
  constructor(options = {}) {
    super();
    this.id = Math.random().toString(36).substr(2, 9);
    this.deviceType = options.deviceType || 'container';
    this.configDir = options.configDir || this.getConfigDir();
    this.connected = false;
    this.server = null;
    
    // 设备状态
    this.state = {
      started_at: null,
      last_heartbeat: null,
      health: 'healthy',
      metadata: {
        environment: process.env.ENV, 
        labels: this.getInitialLabels()
      }
    };
  }
  
  // 初始化 Agent
  async init() {
    console.log(`[Agent] Initializing ${this.deviceType} agent: ${this.id}`);
    
    // 创建配置目录
    this.ensureConfigDir();
    
    // 保存设备类型
    this.saveDeviceType();
    
    // 获取初始标签
    this.state.metadata.labels.push(this.getLabels());
    
    this.state.started_at = new Date().toISOString();
    this.state.last_heartbeat = this.state.started_at;
    
    console.log(`[Agent] Init complete. Device ID: ${this.id}, Type: ${this.deviceType}`);
    this.emit('initialized', this.state);
  }
  
  // 连接 DevOps 服务器
  connect(serverUrl, token) {
    if (this.connected) {
      console.log('[Agent] Already connected');
      return;
    }
    
    console.log('[Agent] Connecting to DevOps server...');
    
    this.server = net.createConnection({
      host: serverUrl,
      port: 3001
    }, () => {
      this.connected = true;
      this.state.connected = true;
      this.emit('connected');
    });
    
    this.server.on('data', (data) => {
      const message = JSON.parse(data);
      console.log(`[Agent] Received: ${message.type}`);
      this.handleServerMessage(message);
    });
    
    this.server.on('error', (err) => {
      console.error('[Agent] Connection error:', err.message);
      this.connected = false;
      
      // 尝试重连
      setTimeout(() => this.connect(serverUrl, token), 5000);
    });
  }
  
  handleServerMessage(message) {
    const { type, payload } = message;
    
    switch (type) {
      case 'SET_CONFIG':
        this.applyConfiguration(payload);
        break;
        
      case 'HEARTBEACK_REQUEST':
        this.sendHeartbeat();
        break;
        
      case 'QUERY_STATUS':
        this.sendStatus({
          state: this.state,
          metrics: this.collectMetrics()
        });
        break;
    }
  }
  
  // 应用配置
  applyConfiguration(config) {
    console.log(`[Agent] Applying config: ${config.command}`);
    
    // 记录配置变更
    this.saveConfig(config);
    
    // 应用配置到设备
    this.applyToDevice(config);
    
    this.emit('config_applied', {
      command: config.command,
      timestamp: new Date().toISOString()
    });
  }
  
  // 发送心跳
  sendHeartbeat() {
    this.state.last_heartbeat = new Date().toISOString();
    this.state.connected = true;
    
    this.emit('heartbeat_sent', { time: new Date().toISOString() });
  }
  
  // 收集指标
  collectMetrics() {
    return {
      uptime_seconds: Math.floor(process.uptime()),
      memory_usage_mb: Math.round((global.heapUsed / 1024 / 1024)),
      label_count: this.state.metadata.labels.length,
      operations_count: this.operationsCount,
      health: this.state.health
    };
  }
  
  // 获取标签
  getLabels() {
    const labels = [
      { env: this.state.metadata.environment },
      { agent_version: '1.0.0' },
      { device_id: this.id }
    ];
    
    // 添加设备类型标签
    this.state.deviceType && labels.push({ device_type: this.deviceType });
    
    return labels;
  }
  
  // 获取初始标签
  getInitialLabels() {
    return [
      { env: process.env.ENV || 'development' }
    ];
  }
  
  // 保存设备类型
  saveDeviceType() {
    const filePath = path.join(this.configDir, 'device_type.json');
    fs.writeFileSync(filePath, JSON.stringify({
      type: this.deviceType,
      timestamp: new Date().toISOString()
    }, null, 2));
  }
  
  // 保存配置
  saveConfig(config) {
    const filePath = path.join(this.configDir, 'applied_config.json');
    
    try {
      const existing = JSON.parse(fs.readFileSync(filePath, 'utf8')) || {};
      existing[config.command] = {
        timestamp: new Date().toISOString(),
        version: config.version || '1.0'
      };
      
      fs.writeFileSync(filePath, JSON.stringify(existing, null, 2));
    } catch (err) {
      this.emit('error', { code: 'SAVE_CONFIG', message: err.message });
    }
  }
  
  // 应用到设备
  applyToDevice(config) {
    console.log(`[Agent] Applying config to device: ${this.id}`);
    
    // 模拟配置应用
    // 实际实现取决于设备类型
    try {
      switch (this.deviceType) {
        case 'network':
          this.applyNetworkConfig(config);
          break;
        default:
          this.applyGenericConfig(config);
      }
      
      this.state.health = 'healthy';
      this.emit('config_applied_successfully', config);
      
    } catch (err) {
      this.state.health = 'unhealthy';
      this.emit('error', { code: 'CONFIG_ERROR', message: err.message });
    }
  }
  
  applyNetworkConfig(config) {
    // 应用网络配置
    console.log(`[Agent] Network config: ${JSON.stringify(config.value)}`);
  }
  
  applyGenericConfig(config) {
    // 应用通用配置
    console.log(`[Agent] Generic config: ${JSON.stringify(config.value)}`);
  }
  
  // 发送心跳
  sendHeartbeat() {
    this.state.last_heartbeat = new Date().toISOString();
    this.emit('heartbeat_sent', { time: new Date().toISOString() });
  }
  
  // 状态报告
  sendStatus(statusPayload) {
    this.emit('status_reported', statusPayload);
  }
  
  // 确保配置目录存在
  ensureConfigDir() {
    if (!fs.existsSync(this.configDir)) {
      fs.mkdirSync(this.configDir, { recursive: true });
    }
  }
  
  // 获取配置目录
  getConfigDir() {
    return path.join(process.env.DOCKER_CONFIG_DIR || '/config', this.id);
  }
  
  // 获取配置目录
  getConfigDir() {
    // 在 Docker 中挂载的配置目录通常位于容器中
    const dockerConfigDir = process.env.DOCKER_CONFIG_DIR || '/opt/devops/config';
    return path.join(dockerConfigDir, this.id);
  }
  
  // 停止服务
  close() {
    if (this.server) {
      this.server.end();
      this.connected = false;
    }
    this.emit('stopped');
  }
  
  // 获取设备状态
  getState() {
    return { ...this.state };
  }
}

module.exports = DevToolsAgent;
