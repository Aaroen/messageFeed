import { defineConfig } from 'vite'

export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:60001',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://127.0.0.1:60001',
        changeOrigin: true,
      },
      '/readyz': {
        target: 'http://127.0.0.1:60001',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://127.0.0.1:60001',
        changeOrigin: true,
      },
    },
  },
})
