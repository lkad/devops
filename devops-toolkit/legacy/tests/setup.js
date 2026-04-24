// Jest setup file for devops-toolkit tests

// Set test environment variables
process.env.NODE_ENV = 'test';
process.env.DEVICE_CONFIG_DIR = '/tmp/devops-test-config';

// Increase timeout for Docker-based tests
jest.setTimeout(10000);

// Mock console.log to reduce noise during tests
global.console = {
  ...console,
  log: jest.fn(),
  debug: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
};

// Clean up after all tests
afterAll(async () => {
  // Allow time for any pending async operations
  await new Promise(resolve => setTimeout(resolve, 100));
});
