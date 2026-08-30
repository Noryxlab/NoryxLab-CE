import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    // Browser targets matter: the 2026-05-04 incident was an untranspiled
    // syntax failure on Safari that silently froze the whole UI.
    target: ['es2022', 'safari16', 'chrome111', 'firefox115', 'edge111'],
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (!id.includes('node_modules')) return undefined;
          if (/[\\/]react-router[\\/]|[\\/]react-dom[\\/]|[\\/]react[\\/]/.test(id)) return 'vendor';
          if (id.includes('@tanstack')) return 'query';
          if (id.includes('keycloak-js')) return 'auth';
          return undefined;
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: process.env.NORYX_API_URL ?? 'http://localhost:8080', changeOrigin: true },
      '/swagger': { target: process.env.NORYX_API_URL ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
});
