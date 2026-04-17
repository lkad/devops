/**
 * Test suite for DevOps Toolkit Log API
 * Can run via: node scripts/run-tests.js [options]
 *
 * Options:
 *   --backend <type>  Test specific backend (local|es|loki)
 *   --url <url>       API base URL (default: http://localhost:3000)
 *   --verbose        Show detailed output
 *   --ci             CI mode (exit on first failure)
 */

const http = require('http');
const https = require('https');

// ===========================================
// Configuration
// ===========================================
const args = process.argv.slice(2);
let baseUrl = 'http://localhost:3000';
let backend = 'local';
let verbose = false;
let ciMode = false;

for (let i = 0; i < args.length; i++) {
    if (args[i] === '--url' && args[i + 1]) baseUrl = args[++i];
    else if (args[i] === '--backend' && args[i + 1]) backend = args[++i];
    else if (args[i] === '--verbose') verbose = true;
    else if (args[i] === '--ci') ciMode = true;
}

// ===========================================
// HTTP Client
// ===========================================
function request(method, path, body = null) {
    return new Promise((resolve, reject) => {
        const url = new URL(baseUrl + path);
        const isHttps = url.protocol === 'https:';
        const lib = isHttps ? https : http;

        const options = {
            hostname: url.hostname,
            port: url.port || (isHttps ? 443 : 80),
            path: url.pathname + url.search,
            method,
            headers: { 'Content-Type': 'application/json' }
        };

        const req = lib.request(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                try {
                    const json = JSON.parse(data);
                    resolve({ status: res.statusCode, data: json });
                } catch {
                    resolve({ status: res.statusCode, data: data });
                }
            });
        });

        req.on('error', reject);
        if (body) req.write(JSON.stringify(body));
        req.end();
    });
}

// ===========================================
// Test Framework
// ===========================================
let passed = 0;
let failed = 0;
let currentSuite = '';

function log(msg, ...args) {
    if (verbose) console.log(msg, ...args);
}

function pass(name) {
    passed++;
    console.log(`  ✓ ${name}`);
}

function fail(name, reason) {
    failed++;
    console.log(`  ✗ ${name}`);
    if (reason) console.log(`    Reason: ${reason}`);
    if (ciMode) process.exit(1);
}

function suite(name) {
    currentSuite = name;
    console.log(`\n${name}`);
    console.log('─'.repeat(50));
}

async function expectTrue(name, fn) {
    try {
        const result = await fn();
        if (result === true || (result && result.success === true)) {
            pass(name);
        } else {
            fail(name, `Expected success but got: ${JSON.stringify(result)}`);
        }
    } catch (e) {
        fail(name, e.message);
    }
}

async function expectFalse(name, fn) {
    try {
        const result = await fn();
        if (result === false || (result && result.success === false)) {
            pass(name);
        } else {
            fail(name, `Expected failure but got: ${JSON.stringify(result)}`);
        }
    } catch (e) {
        fail(name, e.message);
    }
}

async function expectStatus(name, expectedStatus, method, path, body = null) {
    try {
        const result = await request(method, path, body);
        if (result.status === expectedStatus) {
            pass(name);
        } else {
            fail(name, `Expected status ${expectedStatus} but got ${result.status}`);
        }
    } catch (e) {
        fail(name, e.message);
    }
}

// ===========================================
// Tests - Health
// ===========================================
async function testHealth() {
    suite('Health Check');

    await expectTrue('GET /health returns healthy', async () => {
        const res = await request('GET', '/health');
        return res.status === 200 && res.data.status === 'healthy';
    });
}

// ===========================================
// Tests - Log Retention
// ===========================================
async function testRetention() {
    suite('Log Retention API');

    await expectTrue('GET /api/logs/retention returns config', async () => {
        const res = await request('GET', '/api/logs/retention');
        return res.status === 200 && res.data.config;
    });

    await expectTrue('PUT /api/logs/retention updates retention_days', async () => {
        const res = await request('PUT', '/api/logs/retention', { retention_days: 60 });
        return res.status === 200 && res.data.config.retention_days === 60;
    });

    await expectTrue('POST /api/logs/retention/apply triggers cleanup', async () => {
        const res = await request('POST', '/api/logs/retention/apply');
        return res.status === 200 && res.data.success === true;
    });

    await expectTrue('GET /api/logs/backend returns health', async () => {
        const res = await request('GET', '/api/logs/backend');
        return res.status === 200 && res.data.health && res.data.health.healthy !== undefined;
    });
}

// ===========================================
// Tests - Log CRUD
// ===========================================
async function testLogCrud() {
    suite('Log CRUD');

    await expectStatus('GET /api/logs returns 200', 200, 'GET', '/api/logs');

    await expectTrue('POST /api/logs creates log entry', async () => {
        const res = await request('POST', '/api/logs', {
            level: 'info',
            message: 'Test log entry',
            source: 'test-suite'
        });
        return res.status === 201 && res.data.success === true;
    });

    await expectTrue('GET /api/logs?level=error filters correctly', async () => {
        const res = await request('GET', '/api/logs?level=error&limit=10');
        return res.status === 200 && res.data.logs.every(l => l.level === 'error');
    });

    await expectTrue('GET /api/logs/stats returns statistics', async () => {
        const res = await request('GET', '/api/logs/stats');
        return res.status === 200 && res.data.stats && res.data.stats.total !== undefined;
    });
}

// ===========================================
// Tests - Log Generation
// ===========================================
async function testLogGeneration() {
    suite('Log Generation');

    await expectTrue('POST /api/logs/generate creates logs', async () => {
        const res = await request('POST', '/api/logs/generate', { count: 20 });
        return res.status === 200 && res.data.generated === 20;
    });

    await expectTrue('Generated logs appear in query results', async () => {
        const res = await request('GET', '/api/logs?limit=100');
        return res.status === 200 && res.data.total > 0;
    });
}

// ===========================================
// Tests - Alerts
// ===========================================
async function testAlerts() {
    suite('Alert Rules');

    await expectTrue('POST /api/logs/alerts creates alert', async () => {
        const res = await request('POST', '/api/logs/alerts', {
            name: 'Test Alert',
            level: 'error',
            pattern: 'test.*error',
            threshold: 1
        });
        return res.status === 201 && res.data.success === true;
    });

    await expectTrue('GET /api/logs/alerts lists alerts', async () => {
        const res = await request('GET', '/api/logs/alerts');
        return res.status === 200 && Array.isArray(res.data.alerts);
    });

    await expectTrue('GET /api/logs/alerts returns created alert', async () => {
        const res = await request('GET', '/api/logs/alerts');
        return res.status === 200 && res.data.alerts.some(a => a.name === 'Test Alert');
    });
}

// ===========================================
// Tests - Filters
// ===========================================
async function testFilters() {
    suite('Saved Filters');

    await expectTrue('POST /api/logs/filters saves filter', async () => {
        const res = await request('POST', '/api/logs/filters', {
            name: 'Test Filter',
            query: { level: 'error' }
        });
        return res.status === 201 && res.data.success === true;
    });

    await expectTrue('GET /api/logs/filters lists saved filters', async () => {
        const res = await request('GET', '/api/logs/filters');
        return res.status === 200 && Array.isArray(res.data.filters);
    });
}

// ===========================================
// Tests - Devices
// ===========================================
async function testDevices() {
    suite('Device API (Sanity)');

    await expectStatus('GET /api/devices returns 200', 200, 'GET', '/api/devices');

    await expectTrue('POST /api/devices creates device', async () => {
        const id = `test-device-${Date.now()}`;
        const res = await request('POST', '/api/devices', {
            id,
            type: 'server',
            name: 'Test Device',
            labels: ['env:test']
        });
        return res.status === 201 && res.data.success === true;
    });
}

// ===========================================
// Tests - Pipelines
// ===========================================
async function testPipelines() {
    suite('Pipeline API (Sanity)');

    await expectStatus('GET /api/pipelines returns 200', 200, 'GET', '/api/pipelines');
}

// ===========================================
// Main
// ===========================================
async function waitForServer() {
    console.log(`Waiting for server at ${baseUrl}...`);
    for (let i = 0; i < 30; i++) {
        try {
            const res = await request('GET', '/health');
            if (res.status === 200) {
                console.log('Server is ready\n');
                return true;
            }
        } catch { }
        await new Promise(r => setTimeout(r, 1000));
    }
    throw new Error('Server did not become ready');
}

async function main() {
    console.log('╔═══════════════════════════════════════════╗');
    console.log('║   DevOps Toolkit - Log API Test Suite      ║');
    console.log('╚═══════════════════════════════════════════╝');
    console.log(`\nBackend: ${backend}`);
    console.log(`URL: ${baseUrl}\n`);

    try {
        await waitForServer();

        await testHealth();
        await testRetention();
        await testLogCrud();
        await testLogGeneration();
        await testAlerts();
        await testFilters();
        await testDevices();
        await testPipelines();

        console.log('\n' + '═'.repeat(50));
        console.log(`Results: ${passed} passed, ${failed} failed`);
        console.log('═'.repeat(50));

        process.exit(failed > 0 ? 1 : 0);
    } catch (e) {
        console.error('Test suite failed:', e.message);
        process.exit(1);
    }
}

main();