/** @type {import('jest').Config} */
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'jsdom',
  moduleNameMapper: {
    '\\.module\\.css$': '<rootDir>/__mocks__/styleMock.js',
  },
  setupFilesAfterEnv: ['<rootDir>/jest.setup.js'],
  testMatch: ['**/*.test.tsx', '**/*.test.ts'],
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx'],
  transform: {
    '^.+\\.(ts|tsx)$': ['ts-jest', {
      tsconfig: {
        jsx: 'react-jsx',
        esModuleInterop: true,
        allowSyntheticDefaultImports: true,
      }
    }],
  },
}