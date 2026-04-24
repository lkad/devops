#!/usr/bin/env node
/**
 * Network Discovery Test Script
 * Tests the discovery module against the containerlab environment
 */

const NetworkDiscovery = require('../discovery/network_discovery');

async function main() {
  console.log('=== Network Discovery Test ===\n');

  const discovery = new NetworkDiscovery({
    network: '172.30.30.0/24',
    timeout: 2000
  });

  console.log('Starting discovery scan...');
  console.log('This will scan 254 hosts on 172.30.30.0/24\n');

  const startTime = Date.now();

  try {
    const result = await discovery.scan();
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

    console.log(`\n=== Scan Complete (${elapsed}s) ===\n`);
    console.log(`Switches found: ${result.results.switches.length}`);
    console.log(`Servers found: ${result.results.servers.length}\n`);

    if (result.results.switches.length > 0) {
      console.log('Switches:');
      for (const sw of result.results.switches) {
        console.log(`  - ${sw.name} (${sw.ip})`);
      }
      console.log();
    }

    if (result.results.servers.length > 0) {
      console.log('Servers:');
      for (const srv of result.results.servers) {
        console.log(`  - ${srv.name} (${srv.ip})`);
      }
      console.log();
    }

    // Get devices ready for registration
    const devices = discovery.getDiscoveredDevices();
    console.log(`\nDevices ready for registration: ${devices.length}`);
    for (const device of devices) {
      console.log(`  - ${device.name} (${device.type}) - ${device.id}`);
    }

  } catch (err) {
    console.error('Scan failed:', err.message);
  }
}

main();