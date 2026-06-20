import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { existsSync, readFileSync } from 'node:fs'

const apiTarget = process.env.VITE_API_TARGET || 'http://127.0.0.1:60001'
const previewHttpsEnabled = process.env.MESSAGEFEED_PREVIEW_HTTPS === '1'
const previewHttpsKey = process.env.MESSAGEFEED_PREVIEW_HTTPS_KEY || 'certs/messagefeed-preview.key'
const previewHttpsCert = process.env.MESSAGEFEED_PREVIEW_HTTPS_CERT || 'certs/messagefeed-preview.crt'
const hmrClientPort = Number(process.env.VITE_HMR_CLIENT_PORT || '')
const hmrProtocol = process.env.VITE_HMR_PROTOCOL
const hmrHost = process.env.VITE_HMR_HOST
const previewHttps =
  previewHttpsEnabled && existsSync(previewHttpsKey) && existsSync(previewHttpsCert)
    ? {
        key: readFileSync(previewHttpsKey),
        cert: readFileSync(previewHttpsCert),
      }
    : undefined
const apiProxy = {
  '/api': {
    target: apiTarget,
    changeOrigin: true,
  },
  '/healthz': {
    target: apiTarget,
    changeOrigin: true,
  },
  '/readyz': {
    target: apiTarget,
    changeOrigin: true,
  },
  '/metrics': {
    target: apiTarget,
    changeOrigin: true,
  },
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    allowedHosts: true,
    hmr:
      hmrClientPort || hmrProtocol || hmrHost
        ? {
            ...(hmrClientPort ? { clientPort: hmrClientPort } : {}),
            ...(hmrProtocol ? { protocol: hmrProtocol } : {}),
            ...(hmrHost ? { host: hmrHost } : {}),
          }
        : undefined,
    proxy: apiProxy,
  },
  preview: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    allowedHosts: true,
    https: previewHttps,
    proxy: apiProxy,
  },
})
