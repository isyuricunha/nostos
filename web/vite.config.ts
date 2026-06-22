import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig(({ mode }) => ({
  plugins: [svelte()],
  ...(mode === 'test' ? { resolve: { conditions: ['browser'] } } : {}),
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:7000',
      '/health': 'http://localhost:7000'
    }
  },
  test: {
    environment: 'jsdom',
    globals: true
  }
}));
