module.exports = {
  testEnvironment: 'node',
  testMatch: ['<rootDir>/../tests/**/*.test.js'],
  collectCoverageFrom: [
    'devices/**/*.js',
    '!devices/**/*.test.js'
  ],
  coverageDirectory: 'coverage',
  coverageReporters: ['text', 'lcov', 'html'],
  verbose: true,
  testTimeout: 10000,
  setupFilesAfterEnv: ['<rootDir>/../tests/setup.js'],
  modulePathIgnorePatterns: ['<rootDir>/node_modules/']
};
