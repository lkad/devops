#!/usr/bin/env node

/**
 * DevOps Toolkit - Entry Point
 * Main orchestration script for the DevOps agent system
 */

const DevToolsAgent = require('../devices/agent');
const DeviceManager = require('../devices/device_manager');
const config = require('../config/config');

// CLI Flags
const args = process.argv.slice(2);
const command = args[0] || 'start';
const action = args[1] || 'init';

// Main execution
async function main() {
  console.log('\n╔═══════════════════════════════════════════╗');
  console.log('║   DevOps Toolkit - Agent System          ║');
  console.log('╚═══════════════════════════════════════════╝\n');
  
  try {
    switch (command) {
      case 'start':
        await runAgent();
        break;
        
      case 'register-device':
        const deviceId = args[2] || 'auto';
        const deviceManager = new DeviceManager();

        const device = deviceManager.registerDevice({
          id: deviceId,
          type: 'container',
          labels: [{ env: config.env }],
          business_unit: args[3] || 'devops'
        });
        
        console.log(`\n✅ Device registered: ${device.id}`);
        break;
        
      case 'list-devices':
        const deviceManagerList = new DeviceManager();
        const devices = deviceManagerList.getAllDevices();
        console.log('\nRegistered Devices:');
        devices.forEach(d => {
          console.log(`  - ${d.id}: ${d.type} (${d.name})`);
        });
        break;
        
      case 'status':
        await runAgent();
        const agent = new DevToolsAgent({
          deviceType: config.deviceTypes.container
        });
        await agent.init();
        await agent.connect(config.serverUrl);
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        const state = agent.getState();
        console.log('\nAgent Status:');
        console.log(`  ID: ${state.id}`);
        console.log(`  Type: ${state.metadata.device_type}`);
        console.log(`  Health: ${state.health}`);
        console.log(`  Uptime: ${state.uptime_seconds}s`);
        break;
        
      case 'help':
      default:
        console.log('\nUsage:');
        console.log('  devops-toolkit start         - Start agent');
        console.log('  devops-toolkit register      - Register device');
        console.log('  devops-toolkit list          - List devices');
        console.log('  devops-toolkit status        - Show status');
        console.log('  devops-toolkit help          - Show help\n');
    }
  } catch (error) {
    console.error(`\n❌ Error: ${error.message}`);
    process.exit(1);
  }
}

// Run agent
async function runAgent() {
  const agent = new DevToolsAgent({
    deviceType: config.deviceTypes.container
  });
  
  await agent.init();
}

// Execute
main();
