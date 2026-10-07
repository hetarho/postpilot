import { WebGLRenderer, TexturePool, DOMAdapter, WebWorkerAdapter } from 'pixi.js'
import { BrowserLocalComponents } from '@/entities/clip-preview/model/local-components'
import { BrowserCaptionScenePixi } from '@/entities/clip-preview/model/ink-caption-pixi'
import { BrowserFootageResources } from '@/entities/clip-preview/model/browser-footage'
import { readBrowserCompositionSnapshot, type BrowserCompositionSnapshot } from '@/entities/clip-preview/model/browser-composition'
import { clipBrowserEncoderConfig } from '@/entities/clip-preview/model/browser-render-capability'
import { compositeBrowserFrame } from '@/entities/clip-preview/model/composite-video'
import type { BrowserVideoInput } from '@/entities/clip-preview/model/browser-video'
import { NativeFadeBlackSurface } from '@/entities/clip-preview/model/native-fadeblack'
import { measureBrowserBackground } from '@/entities/clip-preview/model/background-sampling'
import { drawMeasuredBrowserComponents } from '@/entities/clip-preview/model/background-paint'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/config/clip-browser-render'
import { CLIP_DESIGN } from '@/entities/clip-design/config/clip-design'
import { CLIP_VIDEO_MEASUREMENT_PHASES } from '@/entities/clip-preview/config/render-measurements'
import { CLIP_AUDIO_PROCESSING } from '@/entities/clip-preview/config/audio-processing'
import { createBoundedMediaOutput, type BoundedMediaOutput } from '@/shared/lib/media/bounded-output'
import { createMp4PacketMux } from '@/shared/lib/media/stream-mp4'
import { MediaPacketWindow } from '@/shared/lib/media/packet-window'
import { MediaPhaseRecorder } from '@/shared/lib/media/phase-metrics'
import { measureMp4Output } from '@/shared/lib/media/measure-mp4'
import { verifyMp4Audio } from '@/shared/lib/media/verify-mp4-audio'
import { Phases, flags, sha256, shaJson, type RunRequest } from './contract'

// Pixi defaults to BrowserAdapter. Select its official OffscreenCanvas Worker
// environment before creating renderer, shader or scene resources in this realm.
DOMAdapter.set(WebWorkerAdapter)

// Isolated diagnostic tagging only, matching the retained ink stress harness.
// The product/shared pool is never cleared by a caption or this adapter.
TexturePool.createTexture = new Proxy(TexturePool.createTexture, {
  apply(target, receiver, args) {
    const texture = Reflect.apply(target, receiver, args)
    texture.source.label = texture.label ?? 'texturePool_diagnostic'
    return texture
  },
})
type Runtime = {
  snapshot: BrowserCompositionSnapshot
  arm: RunRequest['arm']
  local: BrowserLocalComponents
  canvas: OffscreenCanvas
  context: OffscreenCanvasRenderingContext2D
  pixi?: WebGLRenderer
  scene?: BrowserCaptionScenePixi
  fadeBlack?: NativeFadeBlackSurface
}
const namespace = 'postpilot-external-benchmark-candidate-v1'
let runtime: Runtime | undefined
let activeOutput: BoundedMediaOutput | undefined
let activeController: AbortController | undefined
let busy = false
const flagsAll = { ...flags, wholeGpuCompositor: false, productionWorkerPath: false }

async function names() {
  if (!navigator.storage?.getDirectory) return []
  const root = await navigator.storage.getDirectory()
  try {
    const directory = await root.getDirectoryHandle(namespace)
    const values: string[] = []
    for await (const [name] of directory.entries()) values.push(name)
    return values
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotFoundError') return []
    throw error
  }
}
async function release() {
  const before = activeOutput?.measurements()
  await activeOutput?.dispose()
  activeOutput = undefined
  const remaining = await names()
  if (remaining.length) throw Error('BENCHMARK_OWNED_OUTPUT_LEAK')
  return { outputBeforeDispose: before, temporaryNamesAfterDispose: remaining, activeOutput: false }
}
async function cleanup() {
  if (busy) throw Error('BENCHMARK_CLEANUP_WHILE_RUNNING')
  await release()
  if (!runtime) return { runtime: false, temporaryNames: await names() }
  const { local, scene, pixi, canvas, fadeBlack } = runtime
  scene?.destroy(); fadeBlack?.close(); local.destroy()
  const afterLocal = local.measurements(), afterScene = scene?.measurements()
  pixi?.destroy()
  canvas.width = canvas.height = 0
  runtime = undefined
  if (afterLocal.bytes || afterLocal.leases || afterLocal.entries || afterLocal.canvas.effectBytes || afterScene?.nodes)
    throw Error('BENCHMARK_OWNED_COMPONENT_LEAK')
  return { runtime: false, local: afterLocal, scene: afterScene, canvasPixels: 0, temporaryNames: await names() }
}
async function prepare(request: RunRequest, phases: Phases, signal: AbortSignal) {
  // Refuse preview snapshots before resource/encoder/output initialization.
  if (request.snapshot.purpose !== 'export') throw Error('CLIP_SNAPSHOT_EXPORT_PURPOSE_REQUIRED')
  const snapshot = await phases.measure('readSnapshot', () => readBrowserCompositionSnapshot(request.snapshot))
  if (request.temperature === 'cold' && runtime) throw Error('BENCHMARK_COLD_REALM_NOT_FRESH')
  if (request.temperature === 'warm' && (!runtime || runtime.arm !== request.arm || runtime.snapshot.snapshotFingerprint !== snapshot.snapshotFingerprint))
    throw Error('BENCHMARK_WARM_RUNTIME_IDENTITY')
  if (!runtime) {
    await phases.measure('componentCanvasInit', async () => {
      const dimensions = clipBrowserEncoderConfig(snapshot.ratio).video
      const canvas = new OffscreenCanvas(dimensions.width, dimensions.height)
      const context = canvas.getContext('2d', { willReadFrequently: true, alpha: false })
      if (!context) throw Error('BENCHMARK_CANVAS_UNSUPPORTED')
      runtime = { snapshot, arm: request.arm, local: new BrowserLocalComponents(snapshot), canvas, context }
    })
    if (request.arm === 'pixi-scene-canvas2d-hybrid') {
      await phases.measure('pixiGraphicsInit', async () => {
        const dimensions = clipBrowserEncoderConfig(snapshot.ratio).video
        const value = new WebGLRenderer()
        runtime!.pixi = value
        await value.init({
          canvas: new OffscreenCanvas(dimensions.width, dimensions.height),
          width: dimensions.width, height: dimensions.height, resolution: 1,
          backgroundAlpha: 0, antialias: false, preserveDrawingBuffer: true,
          preferWebGLVersion: 2,
        })
        runtime!.scene = new BrowserCaptionScenePixi(value, owner => runtime!.local.dropGPU(owner))
      })
    }
  }
  signal.throwIfAborted()
  // Preserve the first immutable snapshot object, so compatible cached resources
  // share the exact component identities during the explicitly declared warm run.
  await phases.measure('wholeCompositionLayout', () => runtime!.local.resolveLayout(signal))
  return runtime!
}
async function waitEncoder(encoder: VideoEncoder, signal: AbortSignal) {
  if (encoder.encodeQueueSize < CLIP_BROWSER_RENDER.encodeQueueFrames) return
  await new Promise<void>((resolve, reject) => {
    const finish = (error?: unknown) => {
      clearTimeout(timer)
      encoder.removeEventListener('dequeue', check)
      signal.removeEventListener('abort', abort)
      error ? reject(error) : resolve()
    }
    const check = () => { if (encoder.encodeQueueSize < CLIP_BROWSER_RENDER.encodeQueueFrames) finish() }
    const abort = () => finish(signal.reason)
    const timer = setTimeout(() => finish(Error('BENCHMARK_ENCODER_TIMEOUT')), CLIP_BROWSER_RENDER.sourceTimeoutMs)
    encoder.addEventListener('dequeue', check)
    signal.addEventListener('abort', abort, { once: true })
    if (signal.aborted) abort(); else check()
  })
}
async function run(request: RunRequest) {
  if (busy || activeOutput) throw Error('BENCHMARK_CONCURRENT_OR_UNRELEASED_RUN')
  const selected = new Set(request.manifest.sampleFrames)
  if (selected.size !== request.manifest.sampleFrames.length || selected.size > request.manifest.limits.sampleCount ||
    [...selected].some(frame => !Number.isSafeInteger(frame) || frame < 0 || frame >= 450) ||
    request.manifest.limits.outputBytes > CLIP_BROWSER_RENDER.temporaryOutputBytes)
    throw Error('BENCHMARK_BUDGET_INVALID')
  busy = true
  const controller = activeController = new AbortController(), signal = controller.signal
  const phases = new Phases(), workerStarted = performance.now()
  const videoPhases = new MediaPhaseRecorder(CLIP_VIDEO_MEASUREMENT_PHASES)
  let footage: BrowserFootageResources | undefined
  let encoder: VideoEncoder | undefined
  let mux: Awaited<ReturnType<typeof createMp4PacketMux>> | undefined
  let sink = Promise.resolve(), decoderConfig: VideoDecoderConfig | undefined
  let packetCount = 0, presentationValid = true, peakEncodeQueue = 0
  const presentation = new Set<number>(), displayed: unknown[] = [], contracts: unknown[] = []
  const samples: { frame: number; kind: string; rawSha256: string; blob: Blob }[] = []
  let sampleBytes = 0
  const packets = new MediaPacketWindow({
    packets: CLIP_BROWSER_RENDER.encodedPacketWindow,
    bytes: CLIP_BROWSER_RENDER.encodedPacketBytes,
    timeoutMs: CLIP_BROWSER_RENDER.sourceTimeoutMs,
  }, signal)
  const peaks = {
    cpuRasterCacheBytes: 0, cpuCanvasScratchBytes: 0, bitmapReservations: 0,
    gpuManagedTextureBytes: 0, gpuScratchObservedBytes: null as number | null,
    geometryBytes: 0, dynamicShapes: 0, shapeUniformBytes: 0,
    decoderReservedBytes: 0, decodedFrameBytes: 0, decodedFrames: 0,
    finalCanvasLogicalBytes: 0, diagnosticReadbackBytes: 0,
    nativeFadeBlackGpuTextureLowerBoundBytes: 0,
  }
  let frameLive = 0, peakLiveEncoderFrames = 0
  const capture = (readback = 0) => {
    if (!runtime) return
    const c = runtime.local.measurements(), g = runtime.scene?.measurements(), f = footage?.measurements()
    peaks.cpuRasterCacheBytes = Math.max(peaks.cpuRasterCacheBytes, c.bytes)
    peaks.bitmapReservations = Math.max(peaks.bitmapReservations, c.peakBytes)
    peaks.cpuCanvasScratchBytes = Math.max(peaks.cpuCanvasScratchBytes, c.canvas.groupBytes + c.canvas.tintBytes + c.canvas.effectBytes)
    peaks.gpuManagedTextureBytes = Math.max(peaks.gpuManagedTextureBytes, g?.managedTextureBytes ?? 0)
    if (g?.filterScratchBytes !== undefined) peaks.gpuScratchObservedBytes = Math.max(peaks.gpuScratchObservedBytes ?? 0, g.filterScratchBytes)
    peaks.geometryBytes = Math.max(peaks.geometryBytes, g?.geometryBytes ?? 0)
    peaks.dynamicShapes = Math.max(peaks.dynamicShapes, g?.dynamicShapes ?? 0)
    peaks.shapeUniformBytes = Math.max(peaks.shapeUniformBytes, g?.shapeUniformBytes ?? 0)
    peaks.decoderReservedBytes = Math.max(peaks.decoderReservedBytes, f?.peakDecoderReservedBytes ?? 0)
    peaks.decodedFrameBytes = Math.max(peaks.decodedFrameBytes, f?.presentation.peakBytes ?? 0)
    peaks.decodedFrames = Math.max(peaks.decodedFrames, f?.presentation.peakFrames ?? 0)
    peaks.finalCanvasLogicalBytes = runtime.canvas.width * runtime.canvas.height * 4
    peaks.diagnosticReadbackBytes = Math.max(peaks.diagnosticReadbackBytes, readback)
    if (runtime.fadeBlack) peaks.nativeFadeBlackGpuTextureLowerBoundBytes = peaks.finalCanvasLogicalBytes
  }
  const addSample = async (frame: number, kind: string, canvas: OffscreenCanvas, pixels: ImageData) => {
    const rawSha256 = await sha256(new Uint8Array(pixels.data.buffer))
    const blob = await canvas.convertToBlob({ type: 'image/png' })
    sampleBytes += blob.size
    if (sampleBytes > request.manifest.limits.samplePngBytes) throw Error('BENCHMARK_SAMPLE_PNG_LIMIT')
    samples.push({ frame, kind, rawSha256, blob })
  }
  const closeEncoder = () => { if (encoder && encoder.state !== 'closed') encoder.close() }
  signal.addEventListener('abort', closeEncoder, { once: true })
  try {
    const current = await prepare(request, phases, signal)
    const snapshot = current.snapshot, config = clipBrowserEncoderConfig(snapshot.ratio).video
    if (snapshot.frameCount !== 450 || snapshot.plan.durationMs !== 15000) throw Error('BENCHMARK_TIMELINE_CONTRACT')
    const sourceAccess = async (id: string, fingerprint: string, accessSignal: AbortSignal) => {
      accessSignal.throwIfAborted()
      if (id !== 'source' || fingerprint !== request.manifest.source.sha256) throw Error('BENCHMARK_SOURCE_IDENTITY')
      return { kind: 'url' as const, url: request.sourceUrl }
    }
    let backgroundDiagnostics: unknown
    const background = await phases.measure('originalBackgroundSampling', () => measureBrowserBackground(
      snapshot, current.local, sourceAccess, signal, diagnostics => { backgroundDiagnostics = diagnostics },
    ))
    footage = new BrowserFootageResources(snapshot, sourceAccess, signal, {
      opened: ms => videoPhases.record('sourceOpen', ms),
      read: (_bytes, ms) => videoPhases.record('sourceRead', ms),
      decoded: ms => videoPhases.record('sourceDecode', ms),
    })
    activeOutput = await phases.measure('boundedOutputInit', () => createBoundedMediaOutput({
      maxBytes: request.manifest.limits.outputBytes, pageBytes: CLIP_BROWSER_RENDER.outputPageBytes,
      temporary: { namespace, identity: crypto.randomUUID() },
    }, signal))
    mux = await phases.measure('muxInit', () => createMp4PacketMux(activeOutput!, 30, request.audio, signal))
    const support = await phases.measure('encoderSupport', () => VideoEncoder.isConfigSupported(config))
    if (!support.supported) throw Error('BENCHMARK_ENCODER_UNSUPPORTED')
    encoder = new VideoEncoder({
      error: error => controller.abort(error),
      output: (chunk, metadata) => {
        if (signal.aborted) return
        try {
          if (chunk.byteLength > CLIP_BROWSER_RENDER.encodedPacketBytes) throw Error('MEDIA_PACKET_WINDOW_LIMIT')
          if (metadata?.decoderConfig) decoderConfig = metadata.decoderConfig
          const data = new Uint8Array(chunk.byteLength); chunk.copyTo(data)
          const packet = { type: chunk.type, timestamp: chunk.timestamp, duration: chunk.duration ?? 0, data }
          const frame = Math.round(chunk.timestamp * 30 / 1e6)
          presentationValid &&= frame >= 0 && frame < snapshot.frameCount && !presentation.has(frame) &&
            Math.abs(chunk.timestamp - frame * 1e6 / 30) <= 1 && Math.abs(packet.duration - 1e6 / 30) <= 1
          presentation.add(frame); packetCount++
          packets.send(data.byteLength, id => {
            sink = sink.then(() => phases.measure('muxPacketSink', () => mux!.video(packet, metadata?.decoderConfig)))
              .then(() => packets.ack(id), error => { packets.ack(id, error); controller.abort(error) })
          })
          capture()
        } catch (error) { controller.abort(error) }
      },
    })
    encoder.configure(config)
    const frameStarted = performance.now()
    const plan = JSON.parse(JSON.stringify(snapshot.plan)) as BrowserVideoInput['plan']
    const input: BrowserVideoInput = { snapshot, plan, ratio: snapshot.ratio, assets: [] }
    for (let frame = 0; frame < snapshot.frameCount; frame++) {
      signal.throwIfAborted()
      const evaluated = await compositeBrowserFrame(input, {
        context: current.context, footage, measurements: videoPhases,
        assertCurrent: () => signal.throwIfAborted(),
        displayed: positions => displayed.push({ frame, positions }),
        nativeFadeBlack: rest => {
          current.fadeBlack ??= new NativeFadeBlackSurface(config.width, config.height)
          current.fadeBlack.apply(current.canvas, current.context, rest)
        },
        local: async state => {
          const active = background.measurements.filter(m => state.components.some(s =>
            s.component.instanceId === m.instanceId && (s.phraseIndex ?? 0) === m.phraseIndex))
          const resources = await phases.measure('effectsPrepare', () => current.local.prepare(state, signal, id => ({
            accent: CLIP_DESIGN.accent[(state.components.find(s => s.component.instanceId === id)?.component.element.accent ?? '') as keyof typeof CLIP_DESIGN.accent],
            accentWhite: active.some(m => m.instanceId === id && m.accentWhite),
          })))
          try {
            await phases.measure('effectsDrawSubmitAndHybridCopy', () => {
              const painted = resources.map(resource => resource.kind === 'scene' && current.scene ? {
                ...resource,
                draw: (ctx: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D) => {
                  current.scene!.render(resource.scene, undefined, true)
                  ctx.drawImage(current.pixi!.canvas, 0, 0)
                },
              } : resource)
              drawMeasuredBrowserComponents(current.context, snapshot, state, painted, background)
            })
            capture()
            if (selected.has(frame)) await phases.measure('componentContractDiagnostic', async () => contracts.push({
              frame, digest: await shaJson(resources.map(r => ({
                instanceId: r.component.component.instanceId, kind: r.kind, caption: r.caption,
                scene: r.kind === 'scene' ? { bounds: r.scene.bounds, opacity: r.scene.opacity,
                  nodes: r.scene.nodes.map(n => ({ id: n.id, pose: n.pose, documentKey: n.document?.key, rect: n.rect, light: n.light, flames: n.flames, sparks: n.sparks })) } : undefined,
              }))),
            }))
          } finally { resources.forEach(r => r.close()) }
        },
        asset: async () => { throw Error('BENCHMARK_FOREIGN_ASSET') },
        captionFrame: async () => { throw Error('BENCHMARK_SERVER_CAPTION') },
        releaseAssets: () => undefined,
      }, frame)
      if (!evaluated) throw Error('BENCHMARK_FROZEN_FRAME_REQUIRED')
      // Equal final-image diagnostic completion boundary on BOTH arms. No
      // Pixi-only gl.finish; hybrid copy cost is measured above, not hidden.
      const pixels = await phases.measure('finalCanvasReadbackDiagnostic', () => current.context.getImageData(0, 0, config.width, config.height))
      capture(pixels.data.byteLength)
      if (selected.has(frame)) await phases.measure('preEncodeSampleDiagnostic', () => addSample(frame, 'preencode', current.canvas, pixels))
      await phases.measure('packetCreditWait', () => packets.reserve())
      await videoPhases.measureAsync('encodeWait', () => waitEncoder(encoder!, signal))
      signal.throwIfAborted()
      const end = videoPhases.begin('encodeSubmit')
      const picture = new VideoFrame(current.canvas, { timestamp: evaluated.timestampUs, duration: evaluated.durationUs })
      frameLive++; peakLiveEncoderFrames = Math.max(peakLiveEncoderFrames, frameLive)
      try { encoder.encode(picture, { keyFrame: frame % CLIP_BROWSER_RENDER.keyFrameIntervalFrames === 0 }); peakEncodeQueue = Math.max(peakEncodeQueue, encoder.encodeQueueSize) }
      finally { picture.close(); frameLive--; end() }
    }
    phases.add('frameLoopIncludingReadbackDiagnostics', performance.now() - frameStarted)
    await videoPhases.measureAsync('encodeWait', () => encoder!.flush())
    await phases.measure('packetSinkDrain', () => packets.drain())
    await sink
    if (!decoderConfig || packetCount !== snapshot.frameCount || !presentationValid) throw Error('BENCHMARK_ENCODER_PRESENTATION_INVALID')
    const file = await phases.measure('muxFinalizeAndLocalFile', () => mux!.finalize())
    const assembledAtMs = performance.now() - workerStarted
    const outputMeasurements = await phases.measure('outputTrackPacketVerification', () => measureMp4Output(file, 30, snapshot.frameCount, signal))
    const decodedAudio = request.audio ? await phases.measure('decodedAacVerification', () => verifyMp4Audio(file, {
      sampleFrames: request.audio!.sampleFrames, sampleRate: request.audio!.config.sampleRate,
      channels: request.audio!.config.numberOfChannels, maxBytes: CLIP_AUDIO_PROCESSING.maxPcmBytes,
      paddingFrames: CLIP_AUDIO_PROCESSING.encoderPaddingFrames, maxSampleFrames: CLIP_AUDIO_PROCESSING.maxSampleFrames,
    }, signal)) : undefined
    await phases.measure('decodedVideoSampleDiagnostic', async () => {
      const { Input, BlobSource, MP4, VideoSampleSink } = await import('mediabunny')
      const input = new Input({ source: new BlobSource(file), formats: [MP4] })
      const canvas = new OffscreenCanvas(config.width, config.height), context = canvas.getContext('2d', { willReadFrequently: true })!
      try {
        const track = await input.getPrimaryVideoTrack()
        if (!track) throw Error('BENCHMARK_OUTPUT_VIDEO_MISSING')
        const source = new VideoSampleSink(track)
        for (const frame of selected) {
          const sample = await source.getSample(frame / 30)
          if (!sample || Math.abs(sample.timestamp - frame / 30) > 1e-6) { sample?.close(); throw Error('BENCHMARK_OUTPUT_SAMPLE_TIMESTAMP') }
          const picture = sample.toVideoFrame()
          try {
            context.drawImage(picture, 0, 0)
            const pixels = context.getImageData(0, 0, config.width, config.height)
            await addSample(frame, 'decoded', canvas, pixels)
          } finally { picture.close(); sample.close() }
        }
      } finally { canvas.width = canvas.height = 0; input.dispose() }
    })
    capture()
    await footage.dispose()
    const afterFootage = footage.measurements()
    if (afterFootage.activeCuts || afterFootage.decoderReservedBytes || afterFootage.presentation.liveFrames || frameLive || peakEncodeQueue > CLIP_BROWSER_RENDER.encodeQueueFrames)
      throw Error('BENCHMARK_FRAME_OR_DECODER_LEAK')
    const video = {
      compositionVersion: snapshot.schemaVersion ? 'clip-browser-composition-v1' : '',
      snapshotFingerprint: snapshot.authoritativeFingerprint ?? snapshot.snapshotFingerprint,
      localSnapshotFingerprint: snapshot.snapshotFingerprint, speechFingerprint: snapshot.speechFingerprint,
      backgroundEvidence: background, sourceResources: afterFootage,
      config, decoderConfig, chunks: [], packetCount, packetResources: packets.measurements(),
      presentationValid, outputMeasurements, frameCount: snapshot.frameCount, durationUs: 15000000,
    }
    const sourceClockSha256 = await shaJson(displayed), componentContractsSha256 = await shaJson(contracts)
    const report = {
      fixture: request.fixture.id, arm: request.arm, temperature: request.temperature,
      snapshotFingerprint: snapshot.snapshotFingerprint, versions: snapshot.versions,
      snapshotSha256: await shaJson(snapshot), sourceClockSha256, componentContractsSha256,
      sourceContract: snapshot.sources, components: snapshot.components.map(c => ({
        instanceId: c.instanceId, text: c.element.text, startMs: c.startMs, endMs: c.endMs, style: c.componentId,
      })),
      backgroundDigest: background.digest, backgroundDiagnostics, video,
      decodedAudio, audioEncodedIdentity: request.audioEncodedIdentity,
      workerPhases: phases.snapshot(), videoPhases: videoPhases.snapshot(), assembledAtMs,
      managedResources: { ...peaks, encoderQueuePeak: peakEncodeQueue, encoderQueueLimit: CLIP_BROWSER_RENDER.encodeQueueFrames,
        peakLiveOwnedEncoderFrames: peakLiveEncoderFrames, packetWindow: packets.measurements(),
        outputSpool: { kind: activeOutput.kind, ...activeOutput.measurements() }, retainedVideoPackets: 0,
        samplePngBytes: sampleBytes, samplePngLimit: request.manifest.limits.samplePngBytes,
        workerHeapBytes: null, physicalGpuBytes: null, driverMsaaBytes: null, fontWasmHeapBytes: null,
        fullAudioMixPrivatePeakBytes: null,
      },
      frameCleanup: { source: afterFootage, liveEncoderFrames: frameLive, encoderClosedAfterRun: true,
        componentLeases: current.local.measurements().leases, cachedRuntimeRetainedForWarm: true },
      completionBoundary: 'Every frame reads the entire final Canvas2D image before encoder submission; this diagnostic cost is separate and changes production throughput',
      cacheTemperature: 'cold=fresh browser/Worker realm; warm=retained font/WASM/layout/ink/Pixi runtime; source decoders, audio processing, encoder and output spool restart',
      uploadMs: null, privatePromotionExecuted: false, ...flagsAll,
    }
    return { report, file, samples }
  } catch (error) {
    controller.abort(error)
    await sink
    await mux?.cancel().catch(() => undefined)
    await activeOutput?.dispose().catch(() => undefined)
    activeOutput = undefined
    throw error
  } finally {
    closeEncoder(); signal.removeEventListener('abort', closeEncoder)
    await footage?.dispose()
    request.audio?.chunks.splice(0)
    activeController = undefined; busy = false
  }
}
self.onmessage = ({ data }) => {
  const { id, kind, value } = data
  if (kind === 'cancel') {
    activeController?.abort(Error('BENCHMARK_CANCELLED'))
    self.postMessage({ id, value: { cancellationRequested: true } })
    return
  }
  const operation = kind === 'run' ? run(value) : kind === 'release' ? release() : kind === 'cleanup' ? cleanup() : Promise.reject(Error('BENCHMARK_UNKNOWN_REQUEST'))
  void operation.then(result => self.postMessage({ id, value: result }), error => self.postMessage({ id, error: error instanceof Error ? error.message : String(error) }))
}
