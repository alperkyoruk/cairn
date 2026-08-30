import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The Go build embeds dist/ with `//go:embed all:dist`, which fails outright if
// the directory does not exist. A committed placeholder keeps a fresh clone
// buildable with plain `go build`, before anyone has run this build -- but
// emptyOutDir deletes it every time, which is why it could never survive long
// enough to be committed. Put it back after each build, whichever way the build
// was invoked.
const keepDistTracked = {
  name: 'cairn:keep-dist-tracked',
  closeBundle() {
    writeFileSync(resolve(import.meta.dirname, 'dist/.gitkeep'), '')
  },
}

// The build output is embedded into the Go binary by ../web/embed.go, so dist/
// must stay where it is. During development the Vue dev server proxies the API
// to a locally running `cairn`, which keeps cookies same-origin.
export default defineConfig({
  plugins: [vue(), keepDistTracked],
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
