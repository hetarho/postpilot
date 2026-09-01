import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import type { Plugin } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

function preserveRenderBlockingEntry(): Plugin {
  return {
    name: 'preserve-render-blocking-entry',
    transformIndexHtml: {
      order: 'post',
      handler(html) {
        // Vite replaces the source entry tag during build, so restore its standard render token.
        return html.replace(
          /<script\b(?=[^>]*\btype="module")(?=[^>]*\bsrc="[^"]+")(?![^>]*\bblocking=)[^>]*>/,
          (entry) => entry.replace('<script', '<script blocking="render"'),
        )
      },
    },
  }
}

export default defineConfig(({ mode }) => {
  // A single repo-root .env is shared by FE and BE (`cp .env.example .env`). Vite's
  // default envDir is the project root (= frontend/), so raise it explicitly —
  // otherwise VITE_* is never read.
  const envDir = path.resolve(__dirname, '..')
  const env = loadEnv(mode, envDir, '')
  const apiUrl = env.VITE_API_URL || 'http://localhost:7678'

  return {
    envDir,
    plugins: [react(), tailwindcss(), preserveRenderBlockingEntry()],
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
    build: {
      // Kept at the 500 kB default deliberately rather than raised. Route-level splitting
      // brought the entry chunk to 352 kB, so nothing trips it and the build carries no
      // standing warning; raising the number would only hide the next regression. (The
      // HEIC worker is far larger but is built in its own environment and is fetched only
      // when the first HEIC is selected, so it is not what this limit is about.)
      chunkSizeWarningLimit: 500,
    },
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      css: true,
      // Every rendered instant goes through Intl with the machine's own zone (localization/
      // format.ts), which is right in a browser and non-deterministic in a test: an assertion
      // written against a wall clock passes in KST on a contributor's machine and fails in UTC
      // on CI. Pinning the runner to the product's home zone makes those assertions mean one
      // thing everywhere; a test that cares about another zone still sets its own.
      env: { TZ: 'Asia/Seoul' },
    },
  }
})
