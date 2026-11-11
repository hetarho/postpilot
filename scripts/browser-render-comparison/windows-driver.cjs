#!/usr/bin/env node
// Windows Node runs this over stdin; every workspace/dependency path is explicit.
const fs = require('node:fs')
const path = require('node:path')
const { createHash, randomUUID } = require('node:crypto')
const { execFileSync } = require('node:child_process')
const { createRequire } = require('node:module')
const { pathToFileURL } = require('node:url')
const os = require('node:os')
const assert = require('node:assert/strict')

const args = process.argv.slice(2)
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback
const sha = value => createHash('sha256').update(value).digest('hex')
const json = file => JSON.parse(fs.readFileSync(file, 'utf8'))
const flags = { productionQualified: false, renderingQualified: false, hardwareQualified: false,
  humanReleaseReviewComplete: false, analysisQualified: false, voiceQualified: false,
  physicalMemoryQualified: false, timingQualified: false, qualification: false }
const temperatures = ['cold', 'warm']
const cleanupTimeoutMs = 10000
const hostTimeoutGraceMs = 30000
const writeJSON = (file, value) => { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, JSON.stringify(value, null, 2) + '\n') }
const git = (root, ...values) => execFileSync('git', values, { cwd: root, encoding: 'utf8' }).trim()

function workspaceInput(root) {
  const pinnedNode = fs.readFileSync(path.join(root, '.node-version'), 'utf8').trim().replace(/^v/, '')
  if (process.versions.node !== pinnedNode) throw Error(`Use the pinned Node ${pinnedNode}; current Node is ${process.versions.node}`)
  const candidate = path.join(root, 'scripts/browser-render-comparison')
  const fixtureBytes = fs.readFileSync(path.join(candidate, 'fixtures.json'))
  const manifest = JSON.parse(fixtureBytes)
  manifest.manifestSha256 = sha(fixtureBytes)
  const contract = fs.readFileSync(path.join(candidate, 'contract.ts'), 'utf8')
  const declaration = contract.match(/export const ARM_IDS\s*=\s*\[([^\]]+)\]/)
  if (!declaration) throw Error('Cannot read the shared comparison arm list')
  const arms = [...declaration[1].matchAll(/'([^']+)'/g)].map(value => value[1])
  if (!arms.length || arms.some(value => !/^[a-z0-9-]+$/.test(value))) throw Error('Invalid shared arm list')
  for (const source of [manifest.source, manifest.speech]) {
    const bytes = fs.readFileSync(path.join(root, source.path))
    if (bytes.length !== source.bytes || sha(bytes) !== source.sha256) throw Error('Synthetic fixture bytes changed')
  }
  return { candidate, manifest, arms }
}

function originalProductionAdapter(root, candidate) {
  const requireFrontend = createRequire(path.join(root, 'frontend/package.json'))
  const ts = requireFrontend('typescript')
  const comparison = fs.readFileSync(path.join(candidate, 'entry.ts'), 'utf8')
  const original = fs.readFileSync(path.join(root, 'frontend/src/test/browser-media/export-entry.ts'), 'utf8')
  const tree = ts.createSourceFile('entry.ts', comparison, ts.ScriptTarget.ES2023, true)
  const builders = [], freezes = []
  const visit = node => {
    if (ts.isFunctionDeclaration(node) && node.name?.text === 'planFor') builders.push(node.getText(tree))
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === 'freezeBrowserComposition') freezes.push(node.getText(tree))
    ts.forEachChild(node, visit)
  }
  visit(tree)
  assert.equal(builders.length, 1, 'Expected one shared plan builder')
  assert.equal(freezes.length, 1, 'Expected one shared snapshot builder')
  const start = original.indexOf('    const plan: ClipEditPlan =')
  const end = original.indexOf('    const audio = await renderBrowserAudio', start)
  if (start < 0 || end < start) throw Error('Production fixture boundary changed; review the adapter')
  const prefix = `type Fixture = { id: string; ratio: ClipRatio; style: string; narrated: boolean };\nconst manifest = await (await fetch('/candidate-manifest.json')).json();\n${builders[0]}\n`
  const replacement = `    const fixture = request.fixture\n    const plan = planFor(fixture)\n    const speechBytes = fixture.narrated ? await (await fetch(request.speechUrl)).arrayBuffer() : undefined\n    const snapshot = await ${freezes[0]}\n`
  let code = prefix + original.slice(0, start) + replacement + original.slice(end)
  code = code.replace('type Request = {', 'type Request = {\n  fixture: Fixture')
  const returned = '          mode: request.mode,'
  if (code.split(returned).length !== 2) throw Error('Production result boundary changed')
  code = code.replace(returned, '          fixture: fixture.id,\n          snapshotFingerprint: snapshot.snapshotFingerprint,\n' + returned)
  const transpiled = ts.transpileModule(code, { fileName: 'windows-final-entry.ts', reportDiagnostics: true,
    compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } })
  const syntax = transpiled.diagnostics ?? []
  if (syntax.length) throw Error('Generated production adapter has syntax diagnostics')
  return { code: transpiled.outputText, provenance: { comparisonEntrySHA256: sha(comparison), originalExportEntrySHA256: sha(original),
    sharedPlanBuilderSHA256: sha(builders[0]), sharedSnapshotBuilderSHA256: sha(freezes[0]), generatedTypeScriptSHA256: sha(code), generatedAdapterSHA256: sha(transpiled.outputText),
    scope: 'Original export fixture production pipeline with the exact shared comparison plan/snapshot builders; no production module is changed' } }
}

function sourceBinding(root, candidate, manifest, adapter) {
  const names = git(root, 'ls-files', 'frontend/src', 'frontend/public/fonts', 'frontend/package.json', 'package.json', '.node-version', 'pnpm-lock.yaml', 'scripts/browser-render-comparison')
    .split('\n').filter(Boolean)
  const sourceHashes = Object.fromEntries(names.map(name => [name, sha(fs.readFileSync(path.join(root, name)))]))
  return { head: git(root, 'rev-parse', 'HEAD'), sourceHashes, lockfileSHA256: sha(fs.readFileSync(path.join(root, 'pnpm-lock.yaml'))),
    fixtureManifestSHA256: manifest.manifestSha256, driverSHA256: sha(fs.readFileSync(path.join(candidate, 'windows-driver.cjs'))),
    productionAdapter: adapter.provenance }
}

function safeRelative(value) {
  if (path.isAbsolute(value) || value.split(/[\\/]/).some(part => !part || part === '..')) throw Error('Artifact path must stay inside the explicit workspace')
  return value
}

async function serve(root, expectedHead) {
  if (process.platform === 'win32') throw Error('Run --serve from WSL with its Linux Node')
  if (git(root, 'rev-parse', 'HEAD') !== expectedHead || git(root, 'status', '--porcelain=1', '--untracked-files=no')) throw Error('Serve a clean exact --expected-head')
  const { candidate, manifest, arms } = workspaceInput(root)
  const adapter = originalProductionAdapter(root, candidate)
  const artifactRelative = safeRelative(option('--artifact-relative', 'tmp/browser-render-windows') + '/' + randomUUID())
  const artifactRoot = path.join(root, artifactRelative)
  fs.mkdirSync(artifactRoot, { recursive: true })
  const binding = { ...sourceBinding(root, candidate, manifest, adapter), artifactRelative, arms }
  const requireFrontend = createRequire(path.join(root, 'frontend/package.json'))
  const { createServer } = await import(pathToFileURL(requireFrontend.resolve('vite')).href)
  const { parseByteRange } = await import(pathToFileURL(path.join(root, 'scripts/browser-media-report.mjs')).href)
  const allowedArtifacts = new Set()
  for (const fixture of manifest.cases) for (const arm of arms) for (const temperature of temperatures) {
    const prefix = `${fixture.id}--${arm}--${temperature}`
    allowedArtifacts.add(prefix + '.mp4')
    for (const kind of ['preencode', 'decoded']) for (const frame of manifest.sampleFrames) allowedArtifacts.add(`${prefix}--${kind}-${frame}.png`)
  }
  const server = await createServer({ configFile: false, root: path.join(root, 'frontend'), cacheDir: path.join(artifactRoot, 'vite-cache'),
    resolve: { alias: { '@': path.join(root, 'frontend/src') } },
    server: { host: '0.0.0.0', port: Number(option('--port', 0)), watch: null, hmr: false, fs: { allow: [root] } },
    plugins: [{ name: 'windows-render-fixtures',
      resolveId(id) { if (id === '/windows-final-entry.ts') return '\0windows-final-entry.ts' },
      load(id) { if (id === '\0windows-final-entry.ts') return adapter.code },
      configureServer(vite) { vite.middlewares.use((request, response, next) => {
        response.setHeader('Cross-Origin-Opener-Policy', 'same-origin')
        response.setHeader('Cross-Origin-Embedder-Policy', 'require-corp')
        const url = new URL(request.url, 'http://localhost')
        if (url.pathname === '/candidate' || url.pathname === '/production') {
          response.setHeader('Content-Type', 'text/html')
          response.end(`<script type="module" src="${url.pathname === '/candidate' ? '/@fs/' + candidate + '/entry.ts' : '/windows-final-entry.ts'}"></script>`); return
        }
        if (url.pathname === '/candidate-manifest.json' || url.pathname === '/binding') {
          response.setHeader('Content-Type', 'application/json')
          response.end(JSON.stringify(url.pathname === '/binding' ? { ...binding, live: sourceBinding(root, candidate, manifest, adapter) } : manifest)); return
        }
        let file
        if (url.pathname === '/candidate-source') file = path.join(root, manifest.source.path)
        if (url.pathname === '/candidate-speech') file = path.join(root, manifest.speech.path)
        if (url.pathname.startsWith('/candidate-artifact/')) {
          const parts = decodeURIComponent(url.pathname.slice('/candidate-artifact/'.length)).split('/')
          if (parts.length !== 2 || !['chrome', 'edge', 'diagnostic-chromium'].includes(parts[0]) || !allowedArtifacts.has(parts[1])) { response.writeHead(404); response.end(); return }
          file = path.join(artifactRoot, ...parts)
        }
        if (!file) { next(); return }
        if (!fs.existsSync(file)) { response.writeHead(404); response.end(); return }
        const bytes = fs.statSync(file).size
        let range
        try { range = parseByteRange(request.headers.range, bytes) } catch { response.writeHead(416); response.end(); return }
        response.writeHead(range ? 206 : 200, { 'Content-Type': file.endsWith('.png') ? 'image/png' : file.endsWith('.wav') ? 'audio/wav' : 'video/mp4',
          'Content-Length': range ? range.end - range.start + 1 : bytes, 'Accept-Ranges': 'bytes', 'Cache-Control': 'no-store',
          ...(range ? { 'Content-Range': `bytes ${range.start}-${range.end}/${bytes}` } : {}) })
        const stream = fs.createReadStream(file, range ?? {})
        response.on('close', () => stream.destroy()); stream.on('error', () => response.destroy()); stream.pipe(response)
      }) },
    }] })
  await server.listen()
  return { server, binding, baseURL: `http://localhost:${server.httpServer.address().port}` }
}

function projectWindowsDevice(value) {
  const text = value => typeof value === 'string' && value.trim() ? value.trim() : null
  const windows = { osVersion: text(value?.osVersion), osBuild: text(value?.osBuild),
    cpu: Array.isArray(value?.cpu) ? value.cpu.map(text).filter(Boolean) : [],
    memoryBytes: Number.isSafeInteger(value?.memoryBytes) && value.memoryBytes > 0 ? value.memoryBytes : null,
    gpu: Array.isArray(value?.gpu) ? value.gpu.filter(item => text(item?.name)).map(item => ({ name: text(item.name), driverVersion: text(item.driverVersion) })) : [] }
  const complete = windows.osVersion && windows.osBuild && windows.cpu.length && windows.memoryBytes && windows.gpu.length && windows.gpu.every(value => value.driverVersion)
  return { status: complete ? 'observed' : 'unmeasured', windows,
    ...(complete ? {} : { reason: 'Windows WMI projection lacks required OS, CPU, RAM or GPU/driver metadata' }),
    integratedWindowsEvidence: false, lowerResourceEvidence: false }
}

function windowsDevice() {
  if (process.platform !== 'win32') return { status: 'unmeasured', reason: `Current host is ${process.platform}/${process.arch}; no Windows execution or WMI data`, windows: null }
  const command = `$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); $postpilotOs=Get-CimInstance Win32_OperatingSystem; $postpilotCpu=Get-CimInstance Win32_Processor; $postpilotRam=Get-CimInstance Win32_ComputerSystem; $postpilotGpu=Get-CimInstance Win32_VideoController; @{osVersion=$postpilotOs.Version;osBuild=$postpilotOs.BuildNumber;cpu=@($postpilotCpu|ForEach-Object {$_.Name});memoryBytes=$postpilotRam.TotalPhysicalMemory;gpu=@($postpilotGpu|ForEach-Object {@{name=$_.Name;driverVersion=$_.DriverVersion}})}|ConvertTo-Json -Depth 4 -Compress`
  try { return projectWindowsDevice(JSON.parse(execFileSync('powershell.exe', ['-NoLogo', '-NoProfile', '-NonInteractive', '-Command', command], { encoding: 'utf8', timeout: cleanupTimeoutMs }))) }
  catch (error) { return { status: 'unmeasured', reason: `Windows device metadata failed: ${error.message}`, windows: null } }
}

function missingWindows(manifest, arms, reason) {
  const reports = [], comparisons = [], productionFinal = []
  for (const browserLabel of ['chrome', 'edge']) for (const fixture of manifest.cases) for (const temperature of temperatures) {
    for (const arm of arms) reports.push({ browserLabel, fixture: fixture.id, arm, temperature, status: 'not-executed', reason,
      workerPhases: null, managedResources: null, verdict: null, ...flags })
    for (const candidateArm of arms.slice(1)) comparisons.push({ browserLabel, fixture: fixture.id, temperature, controlArm: 'canvas2d-shared', candidateArm,
      status: 'not-executed', reason, pixels: null, pixelAcceptanceThreshold: null, humanReviewed: false, ...flags })
    productionFinal.push({ browserLabel, fixture: fixture.id, temperature, status: 'not-executed', reason, verdict: null, ...flags })
  }
  return { reports, comparisons, productionFinal }
}

function coreAt(root) {
  const lock = fs.readFileSync(path.join(root, 'pnpm-lock.yaml'), 'utf8')
  const versions = [...new Set([...lock.matchAll(/^  playwright-core@([0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?):$/gm)].map(value => value[1]))]
  if (versions.length !== 1) throw Error('Lockfile must identify exactly one Playwright core version')
  return require(path.join(root, 'node_modules/.pnpm', `playwright-core@${versions[0]}`, 'node_modules/playwright-core'))
}

function artifactSink(root, browserLabel, manifest) {
  let active, prefix = '', pngBytes = 0
  fs.mkdirSync(path.join(root, browserLabel), { recursive: true })
  return {
    begin(value) { if (active) throw Error('Unfinished previous artifact'); prefix = value; pngBytes = 0 },
    async chunk(name, ordinal, base64, final) {
      const validPng = manifest.sampleFrames.some(frame => ['preencode', 'decoded'].some(kind => name === `${prefix}--${kind}-${frame}.png`))
      if (name !== prefix + '.mp4' && !validPng) throw Error('Artifact outside current bound run')
      if (!active) {
        if (ordinal !== 0 || final) throw Error('Artifact has no initial chunk')
        active = { name, ordinal: 0, bytes: 0, hash: createHash('sha256'), descriptor: validPng ? fs.openSync(path.join(root, browserLabel, name), 'w') : null }
      }
      if (active.name !== name || ordinal !== active.ordinal++) throw Error('Artifact chunk order changed')
      if (!final) {
        if (typeof base64 !== 'string' || base64.length > Math.ceil(manifest.limits.artifactChunkBytes / 3) * 4) throw Error('Encoded artifact chunk exceeds bound')
        const bytes = Buffer.from(base64, 'base64')
        if (bytes.length > manifest.limits.artifactChunkBytes) throw Error('Artifact chunk exceeds bound')
        active.bytes += bytes.length; active.hash.update(bytes)
        if (validPng) pngBytes += bytes.length
        if (active.bytes > manifest.limits.outputBytes || pngBytes > manifest.limits.samplePngBytes) throw Error('Artifact budget exceeded')
        if (active.descriptor !== null) {
          let offset = 0
          while (offset < bytes.length) {
            const written = fs.writeSync(active.descriptor, bytes, offset, bytes.length - offset)
            if (!written) throw Error('Artifact filesystem write stopped')
            offset += written
          }
        }
        return null
      }
      if (base64 !== '') throw Error('Final artifact message contains unaccounted bytes')
      if (active.descriptor !== null) fs.closeSync(active.descriptor)
      const result = { bytes: active.bytes, sha256: active.hash.digest('hex') }; active = undefined
      return result
    },
    close() { if (active?.descriptor !== null && active?.descriptor !== undefined) fs.closeSync(active.descriptor); active = undefined },
  }
}

async function deadline(operation, milliseconds) {
  let timer
  try { return await Promise.race([operation, new Promise((_, reject) => { timer = setTimeout(() => reject(Error('HOST_BENCHMARK_DEADLINE')), milliseconds) })]) }
  finally { clearTimeout(timer) }
}

async function runClient(root, baseURL, expectedHead, diagnosticExecutable) {
  const { manifest, arms } = workspaceInput(root)
  const initial = await (await fetch(baseURL + '/binding')).json()
  if (initial.head !== expectedHead || initial.live.head !== expectedHead || JSON.stringify(initial.sourceHashes) !== JSON.stringify(initial.live.sourceHashes) ||
    initial.fixtureManifestSHA256 !== manifest.manifestSha256 || initial.lockfileSHA256 !== sha(fs.readFileSync(path.join(root, 'pnpm-lock.yaml'))) ||
    initial.driverSHA256 !== sha(fs.readFileSync(path.join(root, 'scripts/browser-render-comparison/windows-driver.cjs')))) throw Error('WSL server/client source binding differs')
  const { chromium } = coreAt(root)
  const artifactRoot = path.join(root, safeRelative(initial.artifactRelative))
  const report = { version: 1, actualExecutionAttempted: true, actualWindowsExecution: process.platform === 'win32' && !diagnosticExecutable,
    hostPlatform: process.platform, hostArchitecture: process.arch, node: process.version, head: expectedHead, sourceBinding: initial,
    validationHost: diagnosticExecutable ? { platform: process.platform, architecture: process.arch, cpu: os.cpus()[0]?.model, memoryBytes: os.totalmem(), kernel: os.release() } : null,
    device: windowsDevice(), browserProfile: 'Playwright-managed temporary profile; default arguments, no owner profile',
    reports: [], comparisons: [], sessions: [], productionFinal: [], ...flags }
  const browsers = diagnosticExecutable ? [{ name: 'diagnostic-chromium', executable: diagnosticExecutable }] : [
    { name: 'chrome', executable: path.join(process.env.ProgramFiles || 'C:\\Program Files', 'Google/Chrome/Application/chrome.exe') },
    { name: 'edge', executable: path.join(process.env['ProgramFiles(x86)'] || 'C:\\Program Files (x86)', 'Microsoft/Edge/Application/msedge.exe') },
  ]
  report.browserTargets = browsers.map(value => ({ browserLabel: value.name, executable: value.executable, present: fs.existsSync(value.executable) }))
  const caseIds = option('--case', '').split(',').filter(Boolean)
  const cases = caseIds.length ? manifest.cases.filter(fixture => caseIds.includes(fixture.id)) : manifest.cases
  if (!cases.length || caseIds.some(id => !cases.some(fixture => fixture.id === id))) throw Error('Unknown fixture selection')
  const budgetEnd = performance.now() + Math.min(Number(option('--timebox-ms', 90 * 60000)), 90 * 60000)
  try {
    for (const selected of browsers) {
      if (!fs.existsSync(selected.executable)) { report.sessions.push({ browserLabel: selected.name, status: 'not-executed', reason: 'Browser executable is absent', executable: selected.executable }); continue }
      let browser
      try {
        browser = await chromium.launch({ executablePath: selected.executable, headless: !args.includes('--headed') })
        for (const fixture of cases) for (const arm of arms) {
          if (performance.now() >= budgetEnd) continue
          const context = await browser.newContext(), page = await context.newPage()
          const sink = artifactSink(artifactRoot, selected.name, manifest)
          const session = { browserLabel: selected.name, fixture: fixture.id, arm, browserErrors: [], cleanup: null }
          report.sessions.push(session); page.on('pageerror', error => session.browserErrors.push(error.message))
          await page.exposeFunction('candidateArtifact', (...values) => sink.chunk(...values))
          try {
            await page.goto(baseURL + '/candidate'); await page.waitForFunction(() => !!window.benchmarkCandidate)
            const environment = await page.evaluate(() => window.benchmarkCandidate.environment())
            for (const temperature of temperatures) {
              if (performance.now() >= budgetEnd) break
              sink.begin(`${fixture.id}--${arm}--${temperature}`)
              const started = performance.now()
              let value
              try { value = await deadline(page.evaluate(input => window.benchmarkCandidate.run(input.fixture, input.arm, input.temperature), { fixture, arm, temperature }), Math.min(manifest.limits.runTimeoutMs + hostTimeoutGraceMs, budgetEnd - performance.now())) }
              catch (error) { value = { status: error.message.includes('BENCHMARK_WEBGL2_UNAVAILABLE') ? 'not-executed' : 'failed', reason: error.message } }
              if (value.artifacts) for (const sample of value.artifacts.samples) sample.name = selected.name + '/' + sample.name
              report.reports.push({ ...value, browserLabel: selected.name, fixture: fixture.id, arm, temperature, hostObservedElapsedMs: performance.now() - started,
                environment, browserVersion: await browser.version(), browserExecutable: selected.executable, browserFlags: [], competingWorkReported: false, ...flags })
              console.log(JSON.stringify({ browser: selected.name, fixture: fixture.id, arm, temperature, status: value.status, reason: value.reason ?? null }))
              if (value.status !== 'done') break
            }
          } catch (error) { session.error = error.message }
          finally { session.cleanup = await deadline(page.evaluate(() => window.benchmarkCandidate?.cleanup()), cleanupTimeoutMs).catch(error => ({ error: error.message })); sink.close(); await context.close() }
        }
        const context = await browser.newContext(), page = await context.newPage()
        try {
          await page.goto(baseURL + '/candidate'); await page.waitForFunction(() => !!window.benchmarkCandidate)
          for (const fixture of cases) for (const temperature of temperatures) for (const candidateArm of arms.slice(1)) {
            const pair = ['canvas2d-shared', candidateArm].map(arm => report.reports.find(value => value.browserLabel === selected.name && value.fixture === fixture.id && value.arm === arm && value.temperature === temperature && value.status === 'done'))
            if (pair.some(value => !value)) continue
            const [control, candidate] = pair, pixels = []
            for (const left of control.artifacts.samples) {
              const right = candidate.artifacts.samples.find(value => value.frame === left.frame && value.kind === left.kind)
              if (!right) throw Error('Comparison sample missing')
              pixels.push({ frame: left.frame, kind: left.kind, rawSha256Equal: left.rawSha256 === right.rawSha256,
                ...await page.evaluate(input => window.benchmarkCandidate.compare(input.left, input.right), { left: left.name, right: right.name }) })
            }
            const contracts = Object.fromEntries(['snapshotFingerprint', 'snapshotSha256', 'sourceClockSha256', 'componentContractsSha256', 'backgroundDigest'].map(key => [key, control[key] === candidate[key]]))
            report.comparisons.push({ browserLabel: selected.name, fixture: fixture.id, temperature, controlArm: 'canvas2d-shared', candidateArm, status: 'observed', contracts,
              contractComparisonPassed: Object.values(contracts).every(Boolean), normalizedPcmIdentityEqual: JSON.stringify(control.audioObservation?.normalizedPcm) === JSON.stringify(candidate.audioObservation?.normalizedPcm),
              audioEncodedIdentityEqual: JSON.stringify(control.audioEncodedIdentity) === JSON.stringify(candidate.audioEncodedIdentity), pixels, pixelAcceptanceThreshold: null, humanReviewed: false, ...flags })
          }
        } finally { await page.evaluate(() => window.benchmarkCandidate?.cleanup()).catch(() => undefined); await context.close() }
        for (const fixture of cases) {
          if (performance.now() >= budgetEnd) continue
          const context = await browser.newContext(), page = await context.newPage()
          try {
            await page.goto(baseURL + '/production'); await page.waitForFunction(() => !!window.exportFixture)
            for (const temperature of temperatures) {
              if (performance.now() >= budgetEnd) break
              const started = performance.now()
              let value
              try { value = await deadline(page.evaluate(input => window.exportFixture.render(input), { fixture, ratio: fixture.ratio, mode: fixture.narrated ? 'mixed' : 'source',
                url: baseURL + '/candidate-source', fingerprint: manifest.source.sha256, speechUrl: baseURL + '/candidate-speech' }), Math.min(manifest.limits.runTimeoutMs + hostTimeoutGraceMs, budgetEnd - performance.now())) }
              catch (error) { value = { error: error.message } }
              const cleanup = await page.evaluate(() => window.exportFixture.recover()).catch(error => ({ error: error.message }))
              const reference = report.reports.find(run => run.browserLabel === selected.name && run.fixture === fixture.id && run.arm === 'canvas2d-shared' && run.temperature === temperature)
              report.productionFinal.push({ ...value, browserLabel: selected.name, fixture: fixture.id, temperature, status: !value.error && value.verdict?.passed ? 'done' : 'failed',
                elapsedMs: performance.now() - started, sameFrozenSnapshotAsComparison: reference?.snapshotFingerprint === value.snapshotFingerprint,
                browserVersion: await browser.version(), cleanup, warmth: 'Repeated page/runtime; production worker lifecycle is unchanged, not a retained-worker cache claim', ...flags })
              console.log(JSON.stringify({ browser: selected.name, productionFinal: fixture.id, temperature, error: value.error ?? null }))
              if (value.error) break
            }
          } finally { await context.close() }
        }
      } catch (error) { report.sessions.push({ browserLabel: selected.name, error: error.message }) }
      finally { await browser?.close() }
    }
  } finally {
    for (const browser of browsers) {
      const missing = missingWindows({ ...manifest, cases }, arms, 'Browser unavailable, execution failed before this combination, or timebox exhausted')
      for (const value of missing.reports.filter(value => value.browserLabel === 'chrome'))
        if (!report.reports.some(run => run.browserLabel === browser.name && run.fixture === value.fixture && run.arm === value.arm && run.temperature === value.temperature)) report.reports.push({ ...value, browserLabel: browser.name })
      for (const value of missing.productionFinal.filter(value => value.browserLabel === 'chrome'))
        if (!report.productionFinal.some(run => run.browserLabel === browser.name && run.fixture === value.fixture && run.temperature === value.temperature)) report.productionFinal.push({ ...value, browserLabel: browser.name })
      for (const value of missing.comparisons.filter(value => value.browserLabel === 'chrome'))
        if (!report.comparisons.some(run => run.browserLabel === browser.name && run.fixture === value.fixture && run.candidateArm === value.candidateArm && run.temperature === value.temperature)) report.comparisons.push({ ...value, browserLabel: browser.name })
    }
    const final = await (await fetch(baseURL + '/binding')).json()
    report.sourceUnchanged = JSON.stringify(initial.live) === JSON.stringify(final.live) && final.live.head === expectedHead
    report.actualExecution = report.reports.some(value => value.status === 'done') || report.productionFinal.some(value => value.status === 'done')
    report.actualWindowsExecution = process.platform === 'win32' && !diagnosticExecutable && report.actualExecution
    report.windowsIntegratedGpuEvidence = false; report.lowerResourceEvidence = false
    report.limits = ['Only checked-in synthetic inputs; no owner footage, live provider or private-storage promotion.',
      'Software renderers remain software evidence; masked renderers are unknown. Codec/hardware hints are not acceleration proof.',
      'Physical process/driver/private codec/DSP peaks and human output acceptance remain unqualified.',
      'An RTX 2080 Ti observation, when actually identified, is discrete-GPU evidence, not Windows integrated-GPU or lower-resource evidence.']
    writeJSON(path.join(artifactRoot, diagnosticExecutable ? 'mac-transport-validation.json' : 'windows-report.json'), report)
  }
  return report
}

async function main() {
  const root = path.resolve(option('--workspace', process.cwd()))
  const { manifest, arms } = workspaceInput(root)
  if (args.includes('--self-test')) {
    const projected = projectWindowsDevice({ osVersion: 'fixture OS', osBuild: 'fixture build', cpu: ['fixture CPU'], memoryBytes: 1024,
      SerialNumber: 'MUST_NOT_PERSIST', gpu: [{ name: 'fixture GPU', driverVersion: 'fixture driver', PNPDeviceID: 'MUST_NOT_PERSIST' }] })
    assert.equal(projected.status, 'observed')
    assert(!JSON.stringify(projected).includes('MUST_NOT_PERSIST'))
    assert.equal(projectWindowsDevice({ osVersion: null, osBuild: null, cpu: [], memoryBytes: null, gpu: [] }).status, 'unmeasured')
    assert.equal(projectWindowsDevice({ ...projected.windows, gpu: [{ name: 'fixture GPU' }] }).status, 'unmeasured')
    const missing = missingWindows(manifest, arms, 'fixture unavailable')
    assert.equal(missing.reports.length, manifest.cases.length * arms.length * temperatures.length * 2)
    assert(missing.reports.every(value => value.status === 'not-executed' && value.verdict === null && value.managedResources === null))
    console.log(JSON.stringify({ fixtureChecks: 'passed', metadataPrivacyAndMissingEvidence: true, windowsExecution: false, qualification: false })); return
  }
  const expectedHead = option('--expected-head', '')
  if (!expectedHead) throw Error('Pass --expected-head and an explicit --workspace for Windows stdin execution')
  const diagnostic = option('--diagnostic-browser', '')
  if (process.platform !== 'win32' && !args.includes('--serve') && !diagnostic) {
    const device = windowsDevice()
    const report = { version: 1, requestedSourceCommit: expectedHead, actualWindowsExecution: false, actualExecution: false, device,
      ...missingWindows(manifest, arms, device.reason), ...flags }
    const output = path.resolve(option('--output', path.join(root, 'tmp/browser-render-windows-unmeasured.json')))
    writeJSON(output, report); console.log(JSON.stringify({ output, windows: 'unmeasured', reports: report.reports.length, productionFinal: report.productionFinal.length, ...flags })); return
  }
  let hosted
  try {
    if (args.includes('--serve') || diagnostic) {
      hosted = await serve(root, expectedHead)
      console.log(JSON.stringify({ serving: hosted.baseURL, head: hosted.binding.head, artifactRelative: hosted.binding.artifactRelative,
        fixtureManifestSHA256: hosted.binding.fixtureManifestSHA256, arms: hosted.binding.arms, windowsExecution: false }))
      if (!diagnostic) { process.once('SIGINT', () => { void hosted.server.close().then(() => process.exit(0)) }); return }
    }
    const baseURL = hosted?.baseURL ?? option('--base-url', '').replace(/\/$/, '')
    if (!/^http:\/\/localhost:\d+$/.test(baseURL)) throw Error('Use the WSL localhost HTTP origin printed by --serve')
    const report = await runClient(root, baseURL, expectedHead, diagnostic)
    const bad = !report.sourceUnchanged || (report.actualWindowsExecution && report.device.status !== 'observed') || report.reports.some(value => value.status !== 'done' || !value.verdict?.passed) ||
      report.comparisons.some(value => !value.contractComparisonPassed || !value.normalizedPcmIdentityEqual) ||
      report.productionFinal.some(value => value.status !== 'done' || !value.sameFrozenSnapshotAsComparison || value.cleanup?.error || value.cleanup?.after?.length) ||
      report.sessions.some(value => value.error || value.cleanup?.error || value.browserErrors?.length)
    console.log(JSON.stringify({ reports: report.reports.length, comparisons: report.comparisons.length, productionFinal: report.productionFinal.length,
      sourceUnchanged: report.sourceUnchanged, actualWindowsExecution: report.actualWindowsExecution, passedTechnicalChecks: !bad, ...flags }))
    if (bad) process.exitCode = 1
  } finally { if (diagnostic) await hosted?.server.close() }
}

if (require.main === module || process.argv[1] === '-') main().catch(error => { console.error(error); process.exitCode = 1 })
