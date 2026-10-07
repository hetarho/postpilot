import { createRequire } from 'node:module'
import { createHash, randomUUID } from 'node:crypto'
import { createReadStream, createWriteStream, readFileSync, writeFileSync, statSync, mkdirSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { resolve, dirname, basename } from 'node:path'
import { pathToFileURL } from 'node:url'
import { once } from 'node:events'

const candidate = dirname(new URL(import.meta.url).pathname)
const args = process.argv.slice(2)
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback
const sourceRoot = resolve(option('--source-root',resolve(candidate,'../..')))
const dependencyRoot = resolve(option('--dependency-root',sourceRoot))
const git = (...values) => execFileSync('git', values, { cwd: sourceRoot, encoding: 'utf8' }).trim()
const digest = bytes => createHash('sha256').update(bytes).digest('hex')
const hashFile = path => digest(readFileSync(path))
const expectedHead = option('--expected-head', '')
if (!expectedHead) throw Error('Pass --expected-head explicitly; final T600 integration requires rebinding')
const currentHead = git('rev-parse', 'HEAD')
if (currentHead !== expectedHead) throw Error(`Source HEAD mismatch: ${currentHead}`)
const trackedDirty = git('status', '--porcelain=1', '--untracked-files=no')
if (trackedDirty && !args.includes('--allow-dirty')) throw Error('Source checkout has tracked changes; source binding must disclose them')
const fixturePath = resolve(candidate, 'fixtures.json'), fixtureBytes = readFileSync(fixturePath)
const fixtures = JSON.parse(fixtureBytes)
for (const item of [fixtures.source,fixtures.speech]) item.path=resolve(sourceRoot,item.path)
if (hashFile(resolve(dependencyRoot, 'pnpm-lock.yaml')) !== hashFile(resolve(sourceRoot, 'pnpm-lock.yaml')))
  throw Error('Read-only dependency checkout does not have the identical lockfile')
const frontPackage = JSON.parse(readFileSync(resolve(sourceRoot, 'frontend/package.json'), 'utf8'))
const dependencyManifestPaths = Object.keys({ ...frontPackage.dependencies, ...frontPackage.devDependencies })
  .map(name => resolve(dependencyRoot, 'frontend/node_modules', name, 'package.json'))
dependencyManifestPaths.push(resolve(dependencyRoot, 'node_modules/playwright/package.json'))
const dependencyManifestHashes = () => Object.fromEntries(dependencyManifestPaths.map(path => [path, hashFile(path)]))
for (const source of [fixtures.source, fixtures.speech]) {
  if (statSync(source.path).size !== source.bytes || hashFile(source.path) !== source.sha256)
    throw Error(`Synthetic fixture does not match its own exact bytes: ${source.path}`)
}
const sourcePaths = git('ls-files', 'frontend/src', 'frontend/public/fonts', 'frontend/package.json', 'package.json', '.node-version', 'pnpm-lock.yaml', 'backend/internal/clip/browser_render.go')
  .split('\n').filter(path => path && !/\.(test|spec)\./u.test(path) && !path.startsWith('frontend/src/test/'))
const sourceHashes = () => Object.fromEntries(sourcePaths.map(path => [path, hashFile(resolve(sourceRoot, path))]))
const adapterPaths = ['runner.mjs', 'static-check.mjs', 'entry.ts', 'audio-observer.ts', 'worker.ts', 'contract.ts', 'fixtures.json', 'README.md']
const adapterHashes = () => Object.fromEntries(adapterPaths.map(path => [path, hashFile(resolve(candidate, path))]))
const bindingPath = resolve(option('--binding', resolve(sourceRoot, 'tmp/browser-render-comparison/binding.json')))
const binding = {
  version: 1, sourceRoot, dependencyRoot, head: currentHead, trackedDirty,
  fixtureManifestSha256: digest(fixtureBytes), lockfileSha256: hashFile(resolve(sourceRoot, 'pnpm-lock.yaml')),
  sourceHashes: sourceHashes(), adapterHashes: adapterHashes(),
  dependencyManifestHashes: dependencyManifestHashes(),
  fixtures, actualExecution: false, qualification: false,
  preparedAt: new Date().toISOString(),
}
if (args.includes('--prepare')) {
  mkdirSync(dirname(bindingPath), { recursive: true })
  writeFileSync(bindingPath, JSON.stringify(binding, null, 2) + '\n')
  console.log(JSON.stringify({ prepared: true, binding: bindingPath, head: currentHead,
    trackedSourceFiles: sourcePaths.length, fixtureManifestSha256: binding.fixtureManifestSha256,
    actualExecution: false, qualification: false }))
  process.exit(0)
}
if (!args.includes('--execute') || !args.includes('--serial-slot-confirmed'))
  throw Error('Preparation only: actual execution requires both --execute and --serial-slot-confirmed from the coordinator')
const prepared = JSON.parse(readFileSync(bindingPath, 'utf8'))
for (const key of ['sourceRoot', 'dependencyRoot', 'head', 'trackedDirty', 'fixtureManifestSha256', 'lockfileSha256', 'sourceHashes', 'adapterHashes', 'dependencyManifestHashes'])
  if (JSON.stringify(prepared[key]) !== JSON.stringify(binding[key])) throw Error(`Prepared binding changed: ${key}; prepare a new binding first`)

const output = resolve(option('--output', resolve(sourceRoot, 'tmp/browser-render-comparison/runs', randomUUID())))
mkdirSync(output, { recursive: true })
const rootRequire = createRequire(resolve(dependencyRoot, 'package.json'))
const frontRequire = createRequire(resolve(dependencyRoot, 'frontend/package.json'))
const { chromium } = rootRequire('playwright')
const { createServer } = await import(pathToFileURL(frontRequire.resolve('vite')).href)
const { parseByteRange } = await import(pathToFileURL(resolve(sourceRoot, 'scripts/browser-media-report.mjs')).href)
const browserManifest = { ...fixtures, manifestSha256: binding.fixtureManifestSha256 }
const allowedArtifacts = new Set()
const selectedIds = option('--case', '').split(',').filter(Boolean)
const cases = selectedIds.length ? fixtures.cases.filter(value => selectedIds.includes(value.id)) : fixtures.cases
if (!cases.length || selectedIds.some(id => !cases.some(value => value.id === id))) throw Error('Unknown bounded fixture selection')
const server = await createServer({
  configFile: false, root: resolve(sourceRoot, 'frontend'), cacheDir: resolve(output, 'vite-cache'),
  resolve: { alias: { '@': resolve(sourceRoot, 'frontend/src') } },
  server: { host: '127.0.0.1', port: 0, watch: null, hmr: false, fs: { allow: [sourceRoot, dependencyRoot, candidate] } },
  plugins: [{
    name: 'readonly-existing-dependency-resolution',
    enforce: 'pre',
    async resolveId(id, importer, options) {
      if (!importer || ![sourceRoot, candidate].some(root => importer.startsWith(root + '/')) ||
        id.startsWith('.') || id.startsWith('/') || id.startsWith('\0') || id.startsWith('@/') || id.startsWith('#')) return null
      if (dependencyRoot === sourceRoot && !importer.startsWith(candidate + '/')) return null
      // Preserve package subpath/conditional exports rather than replacing
      // @bufbuild/.../codegenv2 or WASM imports with nonexistent directories.
      return this.resolve(id, resolve(dependencyRoot, 'frontend/src/external-diagnostic-resolver.ts'), { ...options, skipSelf: true })
    },
  }, {
    name: 'external-readonly-benchmark-candidate',
    configureServer(vite) {
      vite.middlewares.use((request, response, next) => {
        response.setHeader('Cross-Origin-Opener-Policy', 'same-origin')
        response.setHeader('Cross-Origin-Embedder-Policy', 'require-corp')
        if (request.url === '/candidate') {
          response.setHeader('Content-Type', 'text/html')
          response.end(`<script type="module" src="/@fs/${candidate}/entry.ts"></script>`)
          return
        }
        if (request.url === '/candidate-manifest.json') {
          response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify(browserManifest)); return
        }
        let file
        if (request.url === '/candidate-source') file = fixtures.source.path
        if (request.url === '/candidate-speech') file = fixtures.speech.path
        if (request.url?.startsWith('/candidate-artifact/')) {
          const name = decodeURIComponent(request.url.slice('/candidate-artifact/'.length))
          if (name === basename(name) && allowedArtifacts.has(name)) file = resolve(output, name)
          else { response.writeHead(404); response.end(); return }
        }
        if (!file) { next(); return }
        const bytes = statSync(file).size, range = parseByteRange(request.headers.range, bytes)
        response.writeHead(range ? 206 : 200, {
          'Content-Type': file.endsWith('.png') ? 'image/png' : file.endsWith('.wav') ? 'audio/wav' : 'video/mp4',
          'Content-Length': range ? range.end - range.start + 1 : bytes, 'Cache-Control': 'no-store',
          'Accept-Ranges': 'bytes', ...(range ? { 'Content-Range': `bytes ${range.start}-${range.end}/${bytes}` } : {}),
        })
        const stream = createReadStream(file, range ?? {})
        response.on('close', () => stream.destroy()); stream.on('error', () => response.destroy()); stream.pipe(response)
      })
    },
  }],
})
await server.listen()
let browser
const reports = [], comparisons = [], sessions = []
const arms = ['canvas2d-shared', 'pixi-scene-canvas2d-hybrid']
const streams = new Map()
const aggregateFlags = {
  productionQualified: false, renderingQualified: false, hardwareQualified: false,
  humanReleaseReviewComplete: false, analysisQualified: false, voiceQualified: false,
  physicalMemoryQualified: false, timingQualified: false,
}
try {
  browser = await chromium.launch({
    executablePath: option('--chrome', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'),
    headless: !args.includes('--headed'),
    args: ['--enable-precise-memory-info'],
  })
  for (const fixture of cases) for (const arm of arms) {
    const context = await browser.newContext(), page = await context.newPage()
    const session = { fixture: fixture.id, arm, browserErrors: [], cleanup: null }
    sessions.push(session)
    page.on('pageerror', error => session.browserErrors.push(error.message))
    let activePrefix = ''
    await page.exposeFunction('candidateArtifact', async (name, ordinal, base64, final) => {
      if (name !== basename(name) || !name.startsWith(activePrefix + '.') && !name.startsWith(activePrefix + '--') || !/\.(png|mp4)$/u.test(name))
        throw Error('Artifact identity does not belong to current run')
      let state = streams.get(name)
      if (!state) {
        if (ordinal !== 0 || final) throw Error('Artifact has no initial chunk')
        if (streams.size) throw Error('Only one bounded artifact transfer may be active')
        const keep = name.endsWith('.png') || args.includes('--keep-mp4')
        state = { ordinal: 0, bytes: 0, hash: createHash('sha256'), stream: keep ? createWriteStream(resolve(output, name)) : undefined }
        if (state.stream) state.stream.on('error', error => { state.failure = error })
        streams.set(name, state)
      }
      if (state.failure) throw state.failure
      if (ordinal !== state.ordinal) throw Error('Artifact ordinal mismatch')
      if (final) {
        if (base64) throw Error('Final artifact message has bytes')
        if (state.stream) { state.stream.end(); await once(state.stream, 'finish') }
        streams.delete(name)
        if (name.endsWith('.png')) allowedArtifacts.add(name)
        return { bytes: state.bytes, sha256: state.hash.digest('hex') }
      }
      const bytes = Buffer.from(base64, 'base64')
      if (!bytes.length || bytes.length > fixtures.limits.artifactChunkBytes ||
        state.bytes + bytes.length > (name.endsWith('.mp4') ? fixtures.limits.outputBytes : fixtures.limits.samplePngBytes))
        throw Error('Artifact transfer bound exceeded')
      state.ordinal++; state.bytes += bytes.length; state.hash.update(bytes)
      if (state.stream && !state.stream.write(bytes)) await once(state.stream, 'drain')
      return null
    })
    try {
      await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/candidate`)
      await page.waitForFunction(() => !!window.benchmarkCandidate, { timeout: 30000 })
      const environment = await page.evaluate(() => window.benchmarkCandidate.environment())
      for (const temperature of ['cold', 'warm']) {
        activePrefix = `${fixture.id}--${arm}--${temperature}`
        const start = performance.now()
        let timer
        const result = await Promise.race([
          page.evaluate(({ fixture, arm, temperature }) => window.benchmarkCandidate.run(fixture, arm, temperature), { fixture, arm, temperature })
            .catch(error => ({ status: 'failed', reason: error.message })),
          new Promise(resolve => { timer = setTimeout(() => resolve({ status: 'failed', reason: 'HOST_BENCHMARK_DEADLINE' }), fixtures.limits.runTimeoutMs + 30000) }),
        ]).finally(() => clearTimeout(timer))
        const report = { ...result, fixture: fixture.id, arm, temperature, hostObservedElapsedMs: performance.now() - start,
          environment, browserVersion: await browser.version(), browserLaunch: args.includes('--headed') ? 'headed/default-unverified' : 'headless/default-unverified',
          browserFlags: ['--enable-precise-memory-info'],
          competingWorkReported: args.includes('--competing-work'), sourceBinding: { head: binding.head,
            fixtureManifestSha256: binding.fixtureManifestSha256, lockfileSha256: binding.lockfileSha256 }, ...aggregateFlags }
        reports.push(report)
        writeFileSync(resolve(output, activePrefix + '.json'), JSON.stringify(report, null, 2) + '\n')
        console.log(JSON.stringify({ fixture: fixture.id, arm, temperature, status: result.status, reason: result.reason ?? null }))
        if (result.status !== 'done') {
          if (temperature === 'cold') reports.push({ fixture: fixture.id, arm, temperature: 'warm', status: 'not-executed', reason: 'Cold run failed; warm cache provenance unavailable', ...aggregateFlags })
          break
        }
      }
    } finally {
      session.cleanup = await Promise.race([
        page.evaluate(() => window.benchmarkCandidate?.cleanup()).catch(error => ({ error: error.message })),
        new Promise(resolve => setTimeout(() => resolve({ error: 'HOST_CLEANUP_DEADLINE' }), 10000)),
      ])
      await context.close()
      for (const state of streams.values()) state.stream?.destroy()
      streams.clear()
    }
  }
  const context = await browser.newContext(), page = await context.newPage()
  try {
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/candidate`)
    await page.waitForFunction(() => !!window.benchmarkCandidate)
    for (const fixture of cases) for (const temperature of ['cold', 'warm']) {
      const pair = arms.map(arm => reports.find(report => report.fixture === fixture.id && report.arm === arm && report.temperature === temperature && report.status === 'done'))
      if (pair.some(report => !report)) { comparisons.push({ fixture: fixture.id, temperature, status: 'not-executed', reason: 'Missing successful arm', qualification: false }); continue }
      const [a, b] = pair
      const equalFields = ['snapshotFingerprint', 'snapshotSha256', 'sourceClockSha256', 'componentContractsSha256', 'backgroundDigest']
      const contracts = Object.fromEntries(equalFields.map(field => [field, a[field] === b[field]]))
      const contractComparisonPassed = Object.values(contracts).every(value => value === true)
      const audioEncodedIdentityEqual = JSON.stringify(a.audioEncodedIdentity) === JSON.stringify(b.audioEncodedIdentity)
      const decodedAudioMeasurementsEqual = JSON.stringify(a.decodedAudio) === JSON.stringify(b.decodedAudio)
      const normalizedPcmIdentityEqual = JSON.stringify(a.audioObservation?.normalizedPcm) === JSON.stringify(b.audioObservation?.normalizedPcm)
      const pixels = []
      for (const kind of ['preencode', 'decoded']) for (const frame of fixtures.sampleFrames) {
        const left = a.artifacts.samples.find(value => value.frame === frame && value.kind === kind)
        const right = b.artifacts.samples.find(value => value.frame === frame && value.kind === kind)
        if (!left || !right) throw Error('Expected comparison sample missing')
        pixels.push({ kind, frame, rawSha256Equal: left.rawSha256 === right.rawSha256,
          ...await page.evaluate(({ left, right }) => window.benchmarkCandidate.compare(left, right), { left: left.name, right: right.name }) })
      }
      comparisons.push({ fixture: fixture.id, temperature, status: 'observed', contracts, contractComparisonPassed, audioEncodedIdentityEqual,
        decodedAudioMeasurementsEqual, normalizedPcmIdentityEqual, pixels, pixelAcceptanceThreshold: null, humanReviewed: false, qualification: false })
    }
  } finally { await page.evaluate(() => window.benchmarkCandidate?.cleanup()).catch(() => undefined); await context.close() }
} finally {
  await browser?.close(); await server.close()
  const finalSourceHashes = sourceHashes(), finalAdapterHashes = adapterHashes()
  const sourceUnchanged = JSON.stringify(finalSourceHashes) === JSON.stringify(binding.sourceHashes) && git('rev-parse', 'HEAD') === binding.head
  const adapterUnchanged = JSON.stringify(finalAdapterHashes) === JSON.stringify(binding.adapterHashes)
  const dependencyManifestsUnchanged = JSON.stringify(dependencyManifestHashes()) === JSON.stringify(binding.dependencyManifestHashes)
  const fixturesUnchanged = [fixtures.source, fixtures.speech].every(source => statSync(source.path).size === source.bytes && hashFile(source.path) === source.sha256)
  for (const fixture of cases) for (const arm of arms) for (const temperature of ['cold', 'warm'])
    if (!reports.some(report => report.fixture === fixture.id && report.arm === arm && report.temperature === temperature))
      reports.push({ fixture: fixture.id, arm, temperature, status: 'not-executed', reason: 'Execution stopped before this arm/temperature', ...aggregateFlags })
  const report = {
    version: 1, actualExecution: !!browser, actualExecutionAttempted: true, node: process.version, platform: process.platform, architecture: process.arch,
    sourceRoot, dependencyRoot, head: binding.head, sourceHashes: binding.sourceHashes, adapterHashes: binding.adapterHashes,
    fixtureManifestSha256: binding.fixtureManifestSha256, lockfileSha256: binding.lockfileSha256,
    sourceUnchanged, adapterUnchanged, dependencyManifestsUnchanged, dependencyManifestHashes: binding.dependencyManifestHashes,
    fixturesUnchanged, trackedDirty: binding.trackedDirty,
    reports, comparisons, sessions, ...aggregateFlags,
    scope: 'Complete MP4 diagnostic: production shared Canvas2D composition versus authored Pixi scene over Canvas2D footage/regions, with equal per-frame full Canvas readback; not the production video.worker path or a whole GPU compositor',
    limits: [
      'Four synthetic 15-second steady-caption cases, not the full style/pace/device/voice/release matrix.',
      'Full Canvas readback, sample PNG encoding, contract hashing and artifact transfer are diagnostic costs; do not infer uninstrumented production throughput.',
      'Warm reuses browser/Worker font/WASM/layout/ink/Pixi assets; source decoders, audio processing, encoder and output spool restart.',
      'Managed allocations/reservations exclude driver/MSAA/process/font/WASM/private codec/DSP heaps; absent physical observations stay null.',
      'Output metadata, packet clock, sampled pixels and decoded AAC are recorded; human appearance review and decoded waveform equivalence remain separate gates.',
      'Same-origin localhost fixture access is not HTTPS/private-storage CORS/private promotion verification. Upload is unexecuted/null.',
      'Historical T595/T596/T597 timing receipts are neither rewritten nor counted as current-source execution.',
    ],
  }
  writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2) + '\n')
  writeFileSync(resolve(output, 'binding.json'), JSON.stringify(binding, null, 2) + '\n')
  if (!sourceUnchanged || !adapterUnchanged || !dependencyManifestsUnchanged || !fixturesUnchanged || reports.some(report => report.status !== 'done') ||
    comparisons.some(comparison => comparison.contractComparisonPassed === false) ||
    comparisons.some(comparison => comparison.normalizedPcmIdentityEqual === false) ||
    sessions.some(session => session.cleanup?.error || session.browserErrors.length)) process.exitCode = 1
  console.log(JSON.stringify({ output, sourceUnchanged, adapterUnchanged, fixturesUnchanged, qualification: false }))
}
