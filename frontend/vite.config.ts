import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath } from 'node:url'

// `bun run dev` proxies the API to a running nanit backend. It listens on
// NANIT_HTTP_PORT, 8080 unless set, so point this at wherever yours runs.
const apiTarget = process.env.NANIT_API_URL ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    // The Dockerfile copies this into the Go image as /app/web
    outDir: 'dist',
  },
  server: {
    proxy: { '/api': apiTarget, '/health': apiTarget },
  },
})
