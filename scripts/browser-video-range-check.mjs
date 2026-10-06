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
const fixtures = resolve(option('--fixtures', '/private/tmp/postpilot-browser-media-prep-video'))
const output = resolve(option('--output', 'tmp/browser-video-ranges'))
const executablePath = option('--browser', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
const require = createRequire(resolve(root, 'frontend/package.json'))
const { createServer } = await import(pathToFileURL(require.resolve('vite')).href)
const files = new Map([['cfr', resolve(fixtures, 'original-cfr60.mp4')], ['vfr', resolve(fixtures, 'original-vfr.mp4')]])
const requests = []
const server = await createServer({ configFile: false, root: resolve(root, 'frontend'), cacheDir: resolve(root, 'node_modules/.cache/video-range-check'),
  resolve: { alias: { '@': resolve(root, 'frontend/src') } }, server: { host: '127.0.0.1', port: 0, hmr: false, watch: null },
  plugins: [{ name: 'finite-video-range-fixtures', configureServer(vite) {
    vite.middlewares.use((request, response, next) => {
      response.setHeader('Cross-Origin-Opener-Policy', 'same-origin'); response.setHeader('Cross-Origin-Embedder-Policy', 'require-corp')
      if (request.url === '/__video-range__/') { response.setHeader('Content-Type', 'text/html'); response.end('<input type=file><script type=module src="/src/test/browser-media/video-range-entry.ts"></script>'); return }
      if (!request.url?.startsWith('/__video-range__/file/')) return next()
      const mode = request.url.slice('/__video-range__/file/'.length)
      if (request.method !== 'GET') { response.statusCode = 405; response.end(); return }
      if (mode === 'expired') { response.statusCode = 403; response.end('expired'); return }
      const path = files.get(mode === 'whole' ? 'cfr' : mode)
      if (!path) { response.statusCode = 404; response.end(); return }
      const size = statSync(path).size
      const range = mode === 'whole' ? null : parseByteRange(request.headers.range, size)
      response.writeHead(range ? 206 : 200, { 'Content-Type': 'video/mp4', 'Content-Length': range ? range.end - range.start + 1 : size,
        ...(range ? { 'Content-Range': `bytes ${range.start}-${range.end}/${size}` } : {}), 'Accept-Ranges': 'bytes' })
      requests.push({ mode, method: request.method, range, size })
      const stream = createReadStream(path, range ?? {})
      response.on('close', () => stream.destroy()); stream.pipe(response)
    })
  } }],
})
await server.listen()
const origin = server.resolvedUrls.local[0].replace(/\/$/, '')
const browser = await chromium.launch({ executablePath })
try {
  const page = await browser.newPage(); await page.goto(`${origin}/__video-range__/`)
  await page.waitForFunction(() => !!window.videoRangeFixture)
  const results = []
  const expected = new Map([[500, [0,1,2,3,4,5,6,7]], [750,[0,2,3,5,6,8,9,11]], [1000,[0,2,4,6,8,10,12,14]], [1250,[1,3,6,8,11,13,16,18]], [1500,[1,4,7,10,13,16,19,22]], [2000,[1,5,9,13,17,21,25,29]]])
  for (const rate of expected.keys()) {
    const result = await page.evaluate(({ origin, rate }) => window.videoRangeFixture.cursor({ kind: 'url', url: `${origin}/__video-range__/file/cfr` }, `rate-${rate}`, rate), { origin, rate })
    if (result.error) throw new Error(JSON.stringify(result))
    const offsets = result.selected.map((timestamp) => Math.round((timestamp - 2100000) * 60 / 1_000_000))
    if (JSON.stringify(offsets) !== JSON.stringify(expected.get(rate))) throw new Error(`Native rate selection drift at ${rate}: ${offsets}`)
    if (result.budget.liveFrames !== 0 || result.budget.liveBytes !== 0 || result.decodedResources.liveDecodedFrames !== 0) throw new Error('Leaked video frames')
    results.push(result)
  }
  await page.locator('input').setInputFiles(files.get('cfr'))
  results.push(await page.evaluate(() => window.videoRangeFixture.cursor({ kind: 'blob', blob: document.querySelector('input').files[0] }, 'local-file', 1000)))
  for (const mode of ['vfr', 'whole', 'expired']) {
    const result = await page.evaluate(({ origin, mode }) => window.videoRangeFixture.cursor({ kind: 'url', url: `${origin}/__video-range__/file/${mode}` }, mode, 1000), { origin, mode })
    if (mode === 'whole' && result.error !== 'CLIP_SOURCE_RANGE_UNSUPPORTED') throw new Error('Missing finite-range refusal')
    if (mode === 'expired' && result.error !== 'CLIP_SOURCE_EXPIRED') throw new Error('Missing expired-source refusal')
    results.push(result)
  }
  const fingerprint = createHash('sha256').update(readFileSync(files.get('cfr'))).digest('hex')
  const video = await page.evaluate(({ origin, fingerprint }) => window.videoRangeFixture.render(`${origin}/__video-range__/file/cfr`, fingerprint), { origin, fingerprint })
  if (video.error || video.frameCount !== 94 || video.decodedFrames !== 94 || video.sourceResources.presentation.liveFrames !== 0 || video.sourceResources.decoderReservedBytes !== 0) throw new Error(`Production worker failure: ${JSON.stringify(video)}`)
  results.push({ id: 'production-worker', ...video })
  const cancelled = await page.evaluate(({ origin, fingerprint }) => window.videoRangeFixture.render(`${origin}/__video-range__/file/cfr`, fingerprint, 1), { origin, fingerprint })
  if (cancelled.name !== 'AbortError') throw new Error('Cancellation did not stop production worker')
  results.push({ id: 'production-cancel', ...cancelled })
  const report = { version: 1, qualification: false, browser: await browser.version(), executablePath,
    hardwareUsage: 'unmeasured; default installed browser launch', node: process.version, mediabunny: JSON.parse(readFileSync(resolve(root,'frontend/node_modules/mediabunny/package.json'))).version,
    lockfileSHA256: createHash('sha256').update(readFileSync(resolve(root,'pnpm-lock.yaml'))).digest('hex'),
    fixtures: Object.fromEntries([...files].map(([id,path]) => [id,{ bytes: statSync(path).size, sha256: createHash('sha256').update(readFileSync(path)).digest('hex') }])),
    results, requests, sources: ['https://mediabunny.dev/guide/reading-media-files', 'https://mediabunny.dev/guide/media-sinks'] }
  mkdirSync(output, { recursive: true }); writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ browser: report.browser, mediabunny: report.mediabunny, cases: results.length, passed: true, qualification: false, report: resolve(output,'report.json') }))
} finally { await browser.close(); await server.close() }
