import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// DevOps Toolkit frontend — Vite config.
// Backend is expected on :3000 (dev tier) or :3443 (mTLS). The
// dev server proxies /api and /ws to whichever port VITE_API_TARGET
// points at (env-driven, default :3000).
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://127.0.0.1:3000',
        changeOrigin: true,
      },
      '/ws': {
        target: (process.env.VITE_API_TARGET || 'http://127.0.0.1:3000').replace(/^http/, 'ws'),
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
