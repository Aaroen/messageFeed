import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: './',
  plugins: [vue()],
  resolve: { alias: { '@': new URL('../src', import.meta.url).pathname } },
})
