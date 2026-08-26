import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ mode }) => {
  // A single repo-root .env is shared by FE and BE (`cp .env.example .env`). Vite's
  // default envDir is the project root (= frontend/), so raise it explicitly —
  // otherwise VITE_* is never read.
  const envDir = path.resolve(__dirname, '..')
  const env = loadEnv(mode, envDir, '')
  const apiUrl = env.VITE_API_URL || 'http://localhost:7678'

  return {
    envDir,
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      // 2564 = B-L-O-G on a phone keypad (the api is 7678 = P-O-S-T).
      port: 2564,
      strictPort: true,
      proxy: {
        // The Connect transport uses baseUrl '/api' (shared/api/transport.ts); strip the
        // prefix so '/api/postpilot.v1.HealthService/Ping' reaches the backend at root '/'.
        '/api': { target: apiUrl, changeOrigin: true, rewrite: (p) => p.replace(/^\/api/, '') },
        '/health': { target: apiUrl, changeOrigin: true },
      },
    },
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      css: true,
    },
  }
})
