import { createReadStream, readFileSync, statSync, mkdirSync, writeFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { chromium } from 'playwright'
import { parseByteRange } from './browser-media-report.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const args = process.argv.slice(2)
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback
const fixtures = resolve(option('--fixtures', '/private/tmp/postpilot-browser-media-prep-t601'))
const output = resolve(option('--output', 'tmp/browser-analysis-copies'))
const executablePath = option('--browser', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
const require = createRequire(resolve(root, 'frontend/package.json'))
const { createServer } = await import(pathToFileURL(require.resolve('vite')).href)
const files = new Map([
  ['long', resolve(fixtures, 'long-original.mp4')], ['rotated', resolve(fixtures, 'rotated-original.mp4')], ['silent', resolve(fixtures, 'silent-original.mp4')],
  ['vfr', '/private/tmp/postpilot-browser-media-prep-video/original-vfr.mp4'], ['first-track', '/private/tmp/postpilot-browser-media-prep-video/multiple-video-default-second.mp4'],
  ['delayed-audio', '/private/tmp/postpilot-browser-media-prep-audio/delayed-audio.mp4'],
  ['color', '/private/tmp/postpilot-browser-media-prep-analysis/t598-color/tagged709-red.mp4'],
])
mkdirSync(output, { recursive: true })
const requests = []
const server = await createServer({ configFile: false, root: resolve(root, 'frontend'), cacheDir: resolve(root, 'node_modules/.cache/analysis-copy-check'),
  resolve: { alias: { '@': resolve(root, 'frontend/src') } }, server: { host: '127.0.0.1', port: 0, hmr: false, watch: null },
  plugins: [{ name: 'finite-analysis-fixtures', configureServer(vite) {
    vite.middlewares.use(async (request, response, next) => {
      response.setHeader('Cross-Origin-Opener-Policy', 'same-origin'); response.setHeader('Cross-Origin-Embedder-Policy', 'require-corp')
      if (request.url === '/__analysis__/') { response.setHeader('Content-Type', 'text/html'); response.end('<script type=module src="/src/test/browser-media/analysis-entry.ts"></script>'); return }
      if (request.url?.startsWith('/__analysis__/output/')) {
        const id = request.url.slice('/__analysis__/output/'.length)
        if (request.method !== 'PUT' || !/^[a-z0-9-]+$/.test(id)) { response.statusCode = 400; response.end(); return }
        const chunks = []; let bytes = 0
        for await (const chunk of request) { bytes += chunk.length; if (bytes > 8 * 1024 * 1024) { response.statusCode = 413; response.end(); return }; chunks.push(chunk) }
        writeFileSync(resolve(output, `browser-${id}.mp4`), Buffer.concat(chunks)); response.end(); return
      }
      if (!request.url?.startsWith('/__analysis__/file/')) return next()
      const mode = request.url.slice('/__analysis__/file/'.length)
      if (mode === 'expired') { response.statusCode = 403; response.end(); return }
      const path = files.get(mode === 'whole' ? 'long' : mode)
      if (!path) { response.statusCode = 404; response.end(); return }
      const size = statSync(path).size
      const range = mode === 'whole' ? null : parseByteRange(request.headers.range, size)
      response.writeHead(range ? 206 : 200, { 'Content-Type': 'video/mp4', 'Content-Length': range ? range.end - range.start + 1 : size,
        ...(range ? { 'Content-Range': `bytes ${range.start}-${range.end}/${size}` } : {}), 'Accept-Ranges': 'bytes' })
      requests.push({ mode, range, size }); const stream = createReadStream(path, range ?? {}); response.on('close', () => stream.destroy()); stream.pipe(response)
    })
  } }],
})
await server.listen()
const origin = server.resolvedUrls.local[0].replace(/\/$/, '')
const browser = await chromium.launch({ executablePath })
try {
  const page = await browser.newPage(); await page.goto(`${origin}/__analysis__/`)
  await page.waitForFunction(() => !!window.analysisFixture)
  const results = []
  for (const id of [...files.keys(), 'expired', 'whole', 'cancel']) {
    const path = files.get(id) ?? files.get('long'), fingerprint = createHash('sha256').update(readFileSync(path)).digest('hex')
    const result = await page.evaluate(({ origin, id, fingerprint }) => window.analysisFixture.run(`${origin}/__analysis__/file/${id === 'cancel' ? 'long' : id}`, fingerprint, id), { origin, id, fingerprint })
    results.push({ id, ...result })
    if (id === 'expired' && result.error !== 'CLIP_SOURCE_EXPIRED') throw new Error(JSON.stringify(result))
    if (id === 'whole' && result.error !== 'CLIP_SOURCE_RANGE_UNSUPPORTED') throw new Error(JSON.stringify(result))
    if (id === 'cancel' && result.name !== 'AbortError') throw new Error(JSON.stringify(result))
    if (result.resourceEvents?.some((event) => event.resources.liveDecodedFrames !== 0 || event.resources.liveAudioData !== 0 || event.resources.peakDecodedFrames > 48)) throw new Error('Original decoder resource leak or reserve overflow')
    if (result.copies?.some((copy) => copy.resources.liveDecodedFrames !== 0 || copy.resources.liveAudioData !== 0 || copy.resources.peakDecodedFrames > 48)) throw new Error('Decoder resource leak or reserve overflow')
    if (files.has(id) && result.error) throw new Error(`${id}: ${JSON.stringify(result)}`)
    if (id === 'long' && (result.copies.length !== 2 || result.original.durationMs !== 61000 || result.original.audioRate !== 44100)) throw new Error('Original/seam measurement drift')
    if (id === 'rotated' && (result.original.width !== 90 || result.original.height !== 160)) throw new Error('Rotation geometry drift')
    if (id === 'silent' && result.original.hasAudio) throw new Error('Silent source gained audio')
    if (id === 'vfr' && result.original.cadenceVerified) throw new Error('VFR became verified constant cadence')
    console.log(JSON.stringify({ id, passed: true, copies: result.copies?.length, error: result.error }))
  }
  const report = { version: 1, qualification: false, browser: await browser.version(), executablePath, hardwareUsage: 'unmeasured; default installed Chrome', node: process.version,
    mediabunny: JSON.parse(readFileSync(resolve(root, 'frontend/node_modules/mediabunny/package.json'))).version,
    lockfileSHA256: createHash('sha256').update(readFileSync(resolve(root, 'pnpm-lock.yaml'))).digest('hex'),
    fixtures: Object.fromEntries([...files].map(([id, path]) => [id, { bytes: statSync(path).size, sha256: createHash('sha256').update(readFileSync(path)).digest('hex') }])), results, requests }
  writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ passed: true, cases: results.length, qualification: false, report: resolve(output, 'report.json') }))
} finally { await browser.close(); await server.close() }
