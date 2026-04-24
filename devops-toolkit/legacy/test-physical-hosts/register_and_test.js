const PhysicalHostManager = require('../k8s/physical_host_manager');

async function main() {
    const manager = new PhysicalHostManager({
        poolSize: 5,
        heartbeatInterval: 5000,
        commandTimeout: 10000
    });

    const hosts = [
        { id: 'host-1', hostname: 'prod-web-server-01', ip: 'localhost', port: 2221, username: 'root', authMethod: 'password', credentials: { password: 'test123' } },
        { id: 'host-2', hostname: 'prod-db-server-01', ip: 'localhost', port: 2222, username: 'root', authMethod: 'password', credentials: { password: 'test123' } },
        { id: 'host-3', hostname: 'prod-cache-server-01', ip: 'localhost', port: 2223, username: 'root', authMethod: 'password', credentials: { password: 'test123' } }
    ];

    console.log('\n==========================================');
    console.log('   Registering Physical Hosts');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            const host = manager.registerHost(hostInfo);
            console.log(`✓ Registered: ${host.hostname} (${host.ip}:${host.port})`);
        } catch (err) {
            console.log(`✗ Failed to register ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Testing SSH Connections');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            console.log(`Testing connection to ${hostInfo.hostname}...`);
            await manager.getConnection(hostInfo.id);
            console.log(`✓ SSH connected to ${hostInfo.hostname}`);
        } catch (err) {
            console.log(`✗ SSH failed for ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Running Heartbeat Check');
    console.log('==========================================\n');

    const heartbeatResults = await manager.checkAllHosts();
    heartbeatResults.forEach(r => {
        if (r.error) {
            console.log(`✗ ${r.hostId}: ${r.error}`);
        } else {
            console.log(`✓ ${r.hostId}: ${r.state}`);
        }
    });

    console.log('\n==========================================');
    console.log('   Collecting Metrics');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            console.log(`\n--- ${hostInfo.hostname} ---`);
            const metrics = await manager.collectMetrics(hostInfo.id);

            if (metrics.error) {
                console.log(`  Error: ${metrics.error}`);
            } else {
                console.log(`  CPU:    ${metrics.cpu?.usage ?? 'N/A'}% (${metrics.cpu?.cores ?? 0} cores)`);
                console.log(`  Memory: ${metrics.memory?.usagePercent ?? 0}% (${metrics.memory?.total ?? 0}MB total)`);
                if (metrics.disk?.disks) {
                    metrics.disk.disks.forEach(d => {
                        console.log(`  Disk:   ${d.device} ${d.percent}% used (${d.size})`);
                    });
                }
                console.log(`  Uptime: ${metrics.uptime?.formatted ?? 'N/A'}`);
                console.log(`  Collected at: ${metrics.collectedAt}`);
            }
        } catch (err) {
            console.log(`✗ Metrics failed for ${hostInfo.hostname}: ${err.message}`);
        }
    }

    console.log('\n==========================================');
    console.log('   Service Status');
    console.log('==========================================\n');

    for (const hostInfo of hosts) {
        try {
            const service = await manager.checkService(hostInfo.id, 'ssh');
            console.log(`${hostInfo.hostname} - ssh: ${service.status}`);
        } catch (err) {
            console.log(`${hostInfo.hostname} - ssh: unknown (${err.message})`);
        }
    }

    console.log('\n==========================================');
    console.log('   Host Summary');
    console.log('==========================================\n');

    const summary = manager.getSummary();
    console.log(`Total hosts: ${summary.total}`);
    console.log(`Online: ${summary.byState.online}`);
    console.log(`Offline: ${summary.byState.offline}`);
    console.log(`Connection pool size: ${summary.poolSize}`);

    console.log('\n==========================================');
    console.log('   Get All Hosts Details');
    console.log('==========================================\n');

    const allHosts = manager.getAllHosts();
    allHosts.forEach(h => {
        console.log(`[${h.id}] ${h.hostname}`);
        console.log(`  IP: ${h.ip}:${h.port}, State: ${h.state}`);
        console.log(`  Last heartbeat: ${h.lastHeartbeat || 'Never'}`);
        if (h.metrics?.cpu) {
            console.log(`  CPU: ${h.metrics.cpu.usage}%, Memory: ${h.metrics.memory?.usagePercent}%`);
        }
        console.log('');
    });

    manager.shutdown();
    console.log('\nTest completed.');
}

main().catch(console.error);
