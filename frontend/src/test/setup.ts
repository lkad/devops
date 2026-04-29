import { afterEach, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom'

// Add jest and vitest globals
const jestMock = {
  fn: vi.fn,
  clearAllMocks: () => vi.clearAllMocks(),
  useFakeTimers: () => vi.useFakeTimers(),
  useRealTimers: () => vi.useRealTimers(),
  advanceTimersByTime: (ms: number) => vi.advanceTimersByTime(ms),
  spyOn: vi.spyOn,
} as unknown as typeof jest & {
  clearAllMocks: () => void
  useFakeTimers: () => void
  useRealTimers: () => void
  advanceTimersByTime: (ms: number) => void
}

global.jest = jestMock

afterEach(() => {
  cleanup()
})

// Mock fetch
global.fetch = vi.fn()

// Mock WebSocket
class MockWebSocket {
  onopen: ((event: unknown) => void) | null = null
  onclose: ((event: unknown) => void) | null = null
  onmessage: ((event: unknown) => void) | null = null
  onerror: ((event: unknown) => void) | null = null
  readyState = 1 // OPEN

  constructor(public url: string) {}

  send(_data: string): void {
    // Mock implementation
  }

  close(): void {
    this.readyState = 3 // CLOSED
  }
}

global.WebSocket = MockWebSocket as unknown as typeof WebSocket