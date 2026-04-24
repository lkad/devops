/**
 * Network Discovery Manager
 * Discovers devices on the network via SNMP and SSH
 * Uses pull_registration pattern - DevOps Toolkit actively probes the network
 */

const dgram = require('dgram');
const http = require('http');

// Device types for discovered devices
const DiscoveredDeviceType = {
  SWITCH: 'network_device',  // SNMP-enabled
  SERVER: 'physical_host'     // SSH-enabled
};

// SNMP OIDs for device identification
const SNMP_OIDS = {
  sysDescr: '1.3.6.1.2.1.1.1.0',
  sysName: '1.3.6.1.2.1.1.5.0',
  sysContact: '1.3.6.1.2.1.1.4.0',
  sysLocation: '1.3.6.1.2.1.1.6.0'
};

class NetworkDiscovery {
  constructor(options = {}) {
    this.network = options.network || '172.30.30.0/24';
    this.snmpCommunity = options.snmpCommunity || 'public';
    this.sshPort = options.sshPort || 22;
    this.timeout = options.timeout || 3000;

    // Parse network to get IP range
    const [baseIp, prefixLen] = this.network.split('/');
    this.baseIp = baseIp;
    this.prefixLen = parseInt(prefixLen);

    // Calculate IP range
    this.targets = this.calculateTargets();

    // Discovery results
    this.lastScan = null;
    this.isScanning = false;
  }

  calculateTargets() {
    // For /24 network, scan .1 through .254
    const baseParts = this.baseIp.split('.').map(Number);
    const targets = [];

    for (let i = 1; i <= 254; i++) {
      targets.push(`${baseParts[0]}.${baseParts[1]}.${baseParts[2]}.${i}`);
    }

    return targets;
  }

  /**
   * SNMP GET request via UDP
   */
  snmpGet(host, oid, community = 'public') {
    return new Promise((resolve, reject) => {
      const client = dgram.createSocket('udp4');
      const timeout = setTimeout(() => {
        client.close();
        reject(new Error('Timeout'));
      }, this.timeout);

      // Build SNMP GET request (simplified - just check if host responds)
      // Real SNMP would need a proper library like net-snmp
      const request = this.buildSnmpGet(oid, community);

      client.on('message', (msg) => {
        clearTimeout(timeout);
        client.close();
        resolve(msg);
      });

      client.on('error', (err) => {
        clearTimeout(timeout);
        client.close();
        reject(err);
      });

      try {
        client.send(request, 0, request.length, 161, host);
      } catch (err) {
        clearTimeout(timeout);
        client.close();
        reject(err);
      }
    });
  }

  /**
   * Build a minimal SNMP GET request
   */
  buildSnmpGet(oid, community) {
    // Simplified SNMP v1 GET packet
    // This is a basic implementation - real use would need net-snmp library
    const encoded = Buffer.from([
      0x30, 0x00, // SEQUENCE
    ]);
    return encoded;
  }

  /**
   * Check if host responds to ICMP (ping)
   */
  async pingHost(host) {
    return new Promise((resolve) => {
      const isReachable = false;

      // Use Node's built-in capability or skip ping
      // For containerlab environment, assume hosts are reachable if ports are open
      resolve(true); // Simplified - assume reachable
    });
  }

  /**
   * Check if host has SNMP open on UDP 161
   * Uses a minimal SNMP v1 GET request for sysDescr
   */
  async checkSnmp(host) {
    return new Promise((resolve) => {
      const client = dgram.createSocket('udp4');
      let responded = false;

      const timeoutId = setTimeout(() => {
        if (!responded) {
          client.close();
          resolve(null);
        }
      }, this.timeout);

      client.on('message', () => {
        responded = true;
        clearTimeout(timeoutId);
        client.close();
        resolve({
          host,
          port: 161,
          community: this.snmpCommunity,
          type: 'snmp'
        });
      });

      client.on('error', () => {
        clearTimeout(timeoutId);
        client.close();
        resolve(null);
      });

      // Build a proper SNMP v1 GET request for sysDescr OID
      // This minimal packet should elicit a response from any SNMP agent
      const probe = this.buildSnmpGetRequest('1.3.6.1.2.1.1.1.0', this.snmpCommunity);
      try {
        client.send(probe, 0, probe.length, 161, host);
      } catch {
        clearTimeout(timeoutId);
        client.close();
        resolve(null);
      }
    });
  }

  /**
   * Build a minimal SNMP v1 GET request packet
   */
  buildSnmpGetRequest(oid, community) {
    // SNMP v1 GET request packet structure
    // Version: 0 (v1), Community: public, Request ID: random
    const communityBytes = Buffer.from(community, 'ascii');
    const oidBytes = this.encodeOid(oid);

    // Build the PDU
    const pdu = Buffer.alloc(4 + oidBytes.length);
    pdu[0] = 0xa0; // GET_REQUEST type
    pdu[1] = pdu.length - 2; // Length
    pdu[2] = 0x00; pdu[3] = 0x01; // Request ID (1)
    pdu.writeUInt32BE(0, 4); // Error index

    // Variable binding: OID with NULL value
    const varBind = Buffer.alloc(2 + oidBytes.length + 2);
    varBind[0] = 0x06; // Object Identifier
    varBind[1] = oidBytes.length;
    oidBytes.copy(varBind, 2);
    varBind[varBind.length - 2] = 0x05; // NULL type
    varBind[varBind.length - 1] = 0x00; // NULL value

    // Total packet
    const totalLength = 2 + communityBytes.length + 4 + pdu.length + varBind.length + 2;
    const packet = Buffer.alloc(totalLength);

    let offset = 0;
    packet[offset++] = 0x30; // SEQUENCE
    packet[offset++] = totalLength - 2;
    packet[offset++] = 0x02; packet[offset++] = 0x01; packet[offset++] = 0x00; // Version v1
    packet[offset++] = 0x04; packet[offset++] = communityBytes.length; // Community string
    communityBytes.copy(packet, offset); offset += communityBytes.length;

    packet[offset++] = 0xa0; packet[offset++] = pdu.length;
    pdu.copy(packet, offset); offset += pdu.length;

    packet[offset++] = 0x30; packet[offset++] = varBind.length + 2;
    varBind.copy(packet, offset);

    return packet;
  }

  /**
   * Encode OID into BER format
   */
  encodeOid(oid) {
    const parts = oid.split('.').map(Number);
    const result = [];

    // First byte is special: (first*40) + second
    result.push((parts[0] * 40) + parts[1]);

    // Encode remaining parts using base-128 encoding
    for (let i = 2; i < parts.length; i++) {
      const value = parts[i];
      const bytes = [];
      bytes.push(value & 0x7f);
      value >>= 7;
      while (value > 0) {
        bytes.push((value & 0x7f) | 0x80);
        value >>= 7;
      }
      result.push(...bytes.reverse());
    }

    return Buffer.from(result);
  }

  /**
   * Check if host has SSH open on TCP 22
   */
  async checkSsh(host, port = 22) {
    return new Promise((resolve) => {
      const socket = new require('net').Socket();
      let connected = false;

      const timeout = setTimeout(() => {
        if (!connected) {
          socket.destroy();
          resolve(null);
        }
      }, this.timeout);

      socket.on('connect', () => {
        connected = true;
        clearTimeout(timeout);
        socket.destroy();
        resolve({
          host,
          port,
          type: 'ssh'
        });
      });

      socket.on('error', () => {
        clearTimeout(timeout);
        socket.destroy();
        resolve(null);
      });

      socket.connect(port, host);
    });
  }

  /**
   * Perform full network scan
   */
  async scan() {
    if (this.isScanning) {
      return { success: false, error: 'Scan already in progress' };
    }

    this.isScanning = true;
    const results = {
      switches: [],
      servers: [],
      timestamp: new Date().toISOString()
    };

    console.log(`[Discovery] Starting network scan of ${this.network}`);
    console.log(`[Discovery] Scanning ${this.targets.length} hosts...`);

    // Scan in batches for performance
    const batchSize = 50;
    for (let i = 0; i < this.targets.length; i += batchSize) {
      const batch = this.targets.slice(i, i + batchSize);
      const promises = batch.map(async (host) => {
        try {
          // Check SSH first (servers)
          const sshResult = await this.checkSsh(host);
          if (sshResult) {
            console.log(`[Discovery] Found SSH server at ${host}`);
            results.servers.push({
              ...sshResult,
              ip: host,
              name: `server-${host.split('.').pop()}`,
              discovered_at: new Date().toISOString()
            });
            return;
          }

          // Check SNMP (switches)
          const snmpResult = await this.checkSnmp(host);
          if (snmpResult) {
            console.log(`[Discovery] Found SNMP agent at ${host}`);
            results.switches.push({
              ...snmpResult,
              ip: host,
              name: `switch-${host.split('.').pop()}`,
              discovered_at: new Date().toISOString()
            });
          }
        } catch (err) {
          // Silently ignore individual host errors
        }
      });

      await Promise.all(promises);
    }

    this.isScanning = false;
    this.lastScan = results;

    console.log(`[Discovery] Scan complete: ${results.switches.length} switches, ${results.servers.length} servers`);

    return { success: true, results };
  }

  /**
   * Determine datacenter based on IP address
   * DC1: 172.30.30.11-30, DC2: 172.30.30.31-50
   */
  getDatacenterFromIp(ip) {
    const lastOctet = parseInt(ip.split('.').pop());
    if (lastOctet >= 11 && lastOctet <= 30) return 'dc1';
    if (lastOctet >= 31 && lastOctet <= 50) return 'dc2';
    return 'dc1'; // default
  }

  /**
   * Determine device role based on IP and type
   */
  getDeviceRole(ip, type) {
    const lastOctet = parseInt(ip.split('.').pop());
    if (type === 'network_device') {
      return lastOctet <= 12 || (lastOctet >= 31 && lastOctet <= 32) ? 'core' : 'distribution';
    }
    return lastOctet % 2 === 1 ? 'web' : 'db';
  }

  /**
   * Get devices from last scan in registration-ready format
   */
  getDiscoveredDevices() {
    if (!this.lastScan) {
      return [];
    }

    const devices = [];

    // Add switches as network_device type
    for (const sw of this.lastScan.switches) {
      const datacenter = this.getDatacenterFromIp(sw.ip);
      const role = this.getDeviceRole(sw.ip, 'network_device');
      devices.push({
        id: `clab-${datacenter}-${role}-${sw.ip.split('.').pop()}`,
        type: 'network_device',
        name: `${datacenter}-sw-${role}`,
        labels: [
          { datacenter, role: 'switch', tier: role },
          { protocol: 'SNMP' }
        ],
        metadata: {
          ip: sw.ip,
          port: sw.port,
          discovered_at: sw.discovered_at,
          discovery_method: 'snmp_scan'
        }
      });
    }

    // Add servers as physical_host type
    for (const srv of this.lastScan.servers) {
      const datacenter = this.getDatacenterFromIp(srv.ip);
      const role = this.getDeviceRole(srv.ip, 'physical_host');
      devices.push({
        id: `clab-${datacenter}-${role}-${srv.ip.split('.').pop()}`,
        type: 'physical_host',
        name: `${datacenter}-${role}`,
        labels: [
          { datacenter, role: 'server', tier: role },
          { protocol: 'SSH' }
        ],
        metadata: {
          ip: srv.ip,
          port: srv.port,
          discovered_at: srv.discovered_at,
          discovery_method: 'ssh_scan'
        }
      });
    }

    return devices;
  }

  /**
   * Get last scan status
   */
  getStatus() {
    return {
      isScanning: this.isScanning,
      lastScan: this.lastScan ? {
        timestamp: this.lastScan.timestamp,
        switchesFound: this.lastScan.switches.length,
        serversFound: this.lastScan.servers.length
      } : null
    };
  }
}

module.exports = NetworkDiscovery;