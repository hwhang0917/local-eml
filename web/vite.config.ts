import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:7878', changeOrigin: false },
      '/healthz': 'http://127.0.0.1:7878',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
