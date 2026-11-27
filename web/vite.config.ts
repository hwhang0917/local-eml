import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { gzipSync } from 'node:zlib'
import { thirdPartyNotices } from './third-party/notices'

// The HWP renderer is a 28 MB WASM module. Ship it gzipped (~10 MB) so the
// Go binary that embeds dist/ doesn't grow by the full size; the server
// sends it with Content-Encoding: gzip (internal/server/spa.go).
const gzipWasm = {
  name: 'gzip-wasm',
  apply: 'build' as const,
  closeBundle() {
    const dir = join(__dirname, 'dist', 'assets')
    for (const f of readdirSync(dir)) {
      if (!f.endsWith('.wasm')) continue
      const p = join(dir, f)
      writeFileSync(`${p}.gz`, gzipSync(readFileSync(p), { level: 9 }))
      rmSync(p)
    }
  },
}

export default defineConfig({
  plugins: [vue(), tailwindcss(), gzipWasm, thirdPartyNotices()],
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
