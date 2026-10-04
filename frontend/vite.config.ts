import { readFileSync } from 'node:fs'
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

/** The names a page's index re-exports, each with the module and binding it comes from. */
function pageIndexExports(page: string): Map<string, { local: string; from: string }> {
  const index = readFileSync(path.resolve(__dirname, `src/pages/${page}/index.ts`), 'utf8')
  const names = new Map<string, { local: string; from: string }>()
  for (const [, list, from] of index.matchAll(/export \{([^}]+)\} from '([^']+)'/g))
    for (const spec of list!.split(',')) {
      const [local, exported = local] = spec.trim().split(/\s+as\s+/)
      if (local) names.set(exported!, { local, from: from! })
    }
  return names
}

/** `app/routes` reads a page's search schema through the page's index (ARCH-13, ARCH-16) and
 *  lazily imports the same index for the page itself. Rolldown keeps a module imported both ways
 *  in the importer's chunk, so the schema import alone put each such page — TemplatePage,
 *  GuidelinesPage, the model pages, signup, billing — in the entry. The page indexes are side
 *  effect free (`sideEffects` in package.json), so in the build a route's import of names an
 *  index re-exports from its `model` segment is taken from that module, which is what
 *  tree-shaking the index means, and the index is left to the lazy import. An import naming
 *  anything else — a page routed eagerly — stays as written. Source, lint and tests never see
 *  this. */
function routeSchemasPastPageIndex(): Plugin {
  return {
    name: 'route-schemas-past-page-index',
    apply: 'build',
    enforce: 'pre',
    transform(code, id) {
      if (!/\/src\/app\/routes\/[^/]+\.ts$/.test(id.split('?')[0]!)) return null
      return code.replace(
        /import \{([^}]+)\} from '@\/pages\/([a-z-]+)'/g,
        (statement, list: string, page: string) => {
          const exported = pageIndexExports(page)
          const wanted = list
            .split(',')
            .map((name) => name.trim())
            .filter(Boolean)
            .map((name) => ({ name, source: exported.get(name) }))
          if (wanted.some(({ source }) => !source?.from.startsWith('./model/'))) return statement
          return wanted
            .map(
              ({ name, source }) =>
                `import { ${source!.local} as ${name} } from '@/pages/${page}/${source!.from.slice(2)}'`,
            )
            .join('\n')
        },
      )
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
    plugins: [react(), tailwindcss(), preserveRenderBlockingEntry(), routeSchemasPastPageIndex()],
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
      // Kept at the 500 kB default deliberately rather than raised: raising the number would
      // only hide the next regression. Measured 2026-10-04 the entry chunk is 959 kB (295 kB
      // gzip) and it alone trips the warning — React DOM (≈450 kB before minifying), the router,
      // the providers with every i18n namespace, and the eagerly routed posts, editor and login
      // pages with the features they use. With what it preloads a first visit loads 1.46 MB
      // (460 kB gzip), every lazily routed page outside it. (The HEIC worker is far larger but
      // is built in its own environment and is fetched only when the first HEIC is selected, so
      // it is not what this limit is about.)
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
      // Not an assertion budget: almost every test here mounts the REAL route tree against a
      // fake transport, so a run is CPU-bound and vitest's 5s default starts expiring on a busy
      // machine — in whichever files happen to be scheduled together, not in a failing one. The
      // waits inside the tests are `findBy`/`waitFor`, which resolve as soon as the app does.
      //
      // It stays above the 15s testing-library deadline in `test/setup.ts` so that a wait which
      // never resolves is reported as the query it was, naming the element it wanted, rather
      // than as a test that ran out of time.
      testTimeout: 30_000,
    },
  }
})
