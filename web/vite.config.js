import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The build output is embedded into the Go binary by ../web/embed.go, so dist/
// must stay where it is. During development the Vue dev server proxies the API
// to a locally running `cairn`, which keeps cookies same-origin.
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:7777',
    },
  },
})
