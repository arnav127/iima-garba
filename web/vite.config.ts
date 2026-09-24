import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import { fileURLToPath } from 'node:url';

// The build lands in backend/pb_public so the PocketBase binary serves the app on campus.
// For Vercel, the same build is deployed as a static site (see vercel.json) with VITE_PB_URL set.
export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  envDir: fileURLToPath(new URL('..', import.meta.url)),
  plugins: [preact()],
  build: {
    outDir: fileURLToPath(new URL('../backend/pb_public', import.meta.url)),
    emptyOutDir: true,
    target: 'es2020',
    assetsInlineLimit: 0,
  },
  server: {
    port: 5173,
    host: true,
    proxy: { '/api': { target: 'http://127.0.0.1:8090', changeOrigin: false } },
  },
});
