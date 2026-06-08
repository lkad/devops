import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// DevOps Toolkit frontend — Vite config.
// Backend is expected on :18080 (dev tier). The dev server proxies
// /api -> :18080 so the frontend can use relative paths everywhere
// (per the api-contract spec: "all API paths use relative paths").
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://127.0.0.1:18080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});
