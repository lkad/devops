# 测试服务器容器
const { exec } = require('child_process');

async function testServer(container, port) {
  return new Promise((resolve) => {
    exec(`curl -s -o /dev/null -w "%{http_code}" http://localhost:${port}`, (err, stdout) => {
      resolve({
        container,
        port,
        status: stdout,
        success: stdout === '200'
      });
    });
  });
}

async function runTests() {
  const tests = [
    { container: 'server-web-01', port: 8081 },
    { container: 'server-web-02', port: 8082 },
    { container: 'server-app-01', port: 3000 }
  ];
  
  const results = [];
  
  for (const test of tests) {
    console.log(`Testing ${test.container}:${test.port}...`);
    const result = await testServer(test.container, test.port);
    results.push(result);
    console.log(`${result.success ? 'PASS' : 'FAIL'}: ${result.container}`);
  }
  
  return results.filter(r => r.success);
}

module.exports = { testServer, runTests };
