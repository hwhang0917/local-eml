import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  css: {
    postcss: {
      plugins: [
        {
          // The font packages ship font-display: swap, and the late Pretendard
          // subset swaps reflow every Korean glyph (CLS 0.35 in Lighthouse).
          // 'optional' still uses the font when it loads within the block
          // window — instant on localhost — but never swaps after paint.
          postcssPlugin: 'font-display-optional',
          AtRule: {
            'font-face': (rule: { walkDecls: (name: string, cb: (d: { value: string }) => void) => void }) => {
              rule.walkDecls('font-display', (d) => { d.value = 'optional' })
            },
          },
        },
      ],
    },
  },
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
