/// <reference types="vitest" />
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// Vitest runs in a jsdom environment so React Testing Library
// can render. We split the config from vite.config.ts so the
// dev server keeps its proxy rules (the test env does not need
// them — tests mock fetch with vi.fn()).
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
});
