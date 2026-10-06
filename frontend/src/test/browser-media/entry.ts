import {
  BrowserOriginals,
  renderBrowserAudio,
  renderBrowserVideo,
} from '@/features/render-clip-browser'
import { clipBrowserEncoderConfig, previewMotion } from '@/entities/clip-preview'
import type { CaptionFramePage, PreparedAsset } from '@/entities/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan'
import type { ClipRatio } from '@/entities/clip-project'
import { muxMp4 } from '@/shared/lib/media'
import { ALL_FORMATS, BlobSource, Input, VideoSampleSink } from 'mediabunny'

interface Fixture {
  id: string
  ratio: ClipRatio
  durationMs: number
  prepared: boolean
  audio: 'none' | 'source' | 'narration' | 'mixed'
  plan: ClipEditPlan
  source: { file: string; fingerprint: string }
  nativeAssetPreparationMs?: number
  assets: (Omit<PreparedAsset, 'url'> & { file: string })[]
  sheets: Record<string, (Omit<CaptionFramePage, 'sheet'> & { file: string })[]>
}

interface BenchmarkOutput {
  bytes: number
  sha256: string
  width: number
  height: number
  codec: string | null
  codecParameterString: string | null
  frameCount: number
  durationMs: number
  sampledFrames: number
}

type BenchmarkResult =
  | { status: 'missing-fixture' | 'not-implemented' | 'unsupported'; reason: string }
  | {
      status: 'done' | 'failed'
      reason?: string
      elapsedMs: number
      phases: Record<string, number | null>
      videoMeasurements?: unknown
      peakResources: { mainThreadJSHeapBytes: number | null; workerHeapBytes: null; gpuBytes: null }
      output?: BenchmarkOutput
    }

interface BenchmarkAdapter {
  run: (fixture: Fixture) => Promise<BenchmarkResult>
}

declare global {
  interface Window {
    browserMediaBenchmark: {
      ready: boolean
      environment: () => Promise<unknown>
      run: (fixtureId: string, renderer: string) => Promise<BenchmarkResult>
      register: (renderer: string, adapter: BenchmarkAdapter) => void
      motion: typeof previewMotion
      verifyOutput: (fixtureId: string, blob: Blob) => Promise<BenchmarkOutput>
    }
    browserMediaArtifact?: (fixtureId: string, ordinal: number, base64: string) => Promise<void>
  }
}

const artifactURL = (file: string) => `/__browser-media-fixtures__/${file}`
const fixtures: { cases: Fixture[] } = await fetch(artifactURL('manifest.json')).then(
  (response) => {
    if (!response.ok) throw new Error('Benchmark fixtures unavailable')
    return response.json()
  },
)
const adapters = new Map<string, BenchmarkAdapter>()

function heapBytes(): number | null {
  return (
    (performance as Performance & { memory?: { usedJSHeapSize: number } }).memory?.usedJSHeapSize ??
    null
  )
}

async function environment() {
  const canvas = document.createElement('canvas')
  const gl = canvas.getContext('webgl2')
  const extension = gl?.getExtension('WEBGL_debug_renderer_info')
  const renderer = extension ? String(gl!.getParameter(extension.UNMASKED_RENDERER_WEBGL)) : null
  gl?.getExtension('WEBGL_lose_context')?.loseContext()
  const codecSupport: Record<string, { video: boolean; audio: boolean }> = {}
  for (const ratio of ['vertical', 'horizontal', 'square'] as const) {
    const config = clipBrowserEncoderConfig(ratio)
    codecSupport[ratio] = {
      video:
        typeof VideoEncoder !== 'undefined' &&
        (await VideoEncoder.isConfigSupported(config.video)).supported === true,
      audio:
        typeof AudioEncoder !== 'undefined' &&
        (await AudioEncoder.isConfigSupported(config.audio)).supported === true,
    }
  }
  return {
    userAgent: navigator.userAgent,
    hardwareConcurrency: navigator.hardwareConcurrency,
    reportedDeviceMemoryGB:
      (navigator as Navigator & { deviceMemory?: number }).deviceMemory ?? null,
    offscreenCanvas: typeof OffscreenCanvas !== 'undefined',
    webglProbeRenderer: renderer,
    graphicsProbe: renderer?.match(/swiftshader|llvmpipe|software/i)
      ? 'software'
      : renderer
        ? 'unverified-hardware'
        : 'unknown',
    actualCodecHardwareUse: 'unverified',
    codecSupport,
  }
}

/** Every adapter is verified by this same product-version demux/decoder. */
async function verifyBenchmarkOutput(fixture: Fixture, blob: Blob): Promise<BenchmarkOutput> {
  if (window.browserMediaArtifact) {
    for (let start = 0, ordinal = 0; start < blob.size; start += 1024 * 1024, ordinal++) {
      const bytes = new Uint8Array(await blob.slice(start, start + 1024 * 1024).arrayBuffer())
      let binary = ''
      for (let offset = 0; offset < bytes.length; offset += 8192)
        binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192))
      await window.browserMediaArtifact(fixture.id, ordinal, btoa(binary))
    }
  }
  const config = clipBrowserEncoderConfig(fixture.ratio)
  const input = new Input({ source: new BlobSource(blob), formats: ALL_FORMATS })
  try {
    const track = await input.getPrimaryVideoTrack()
    if (!track) throw new Error('BENCHMARK_OUTPUT_VIDEO_MISSING')
    const durationMs = (await track.computeDuration()) * 1000
    const stats = await track.computePacketStats()
    const sink = new VideoSampleSink(track)
    let sampledFrames = 0
    for (const time of [0, fixture.durationMs / 2000, (fixture.durationMs - 1000 / 30) / 1000]) {
      const sample = await sink.getSample(time)
      if (!sample) throw new Error(`BENCHMARK_OUTPUT_FRAME_MISSING_AT_${time}`)
      sample.close()
      sampledFrames++
    }
    if (
      track.displayWidth !== config.video.width ||
      track.displayHeight !== config.video.height ||
      Math.abs(durationMs - fixture.durationMs) > 1 ||
      stats.packetCount !== Math.round((fixture.durationMs * 30) / 1000)
    )
      throw new Error('BENCHMARK_OUTPUT_CONTRACT_MISMATCH')
    const hash = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
    return {
      bytes: blob.size,
      sha256: [...new Uint8Array(hash)]
        .map((value) => value.toString(16).padStart(2, '0'))
        .join(''),
      width: track.displayWidth,
      height: track.displayHeight,
      codec: track.codec,
      codecParameterString: await track.getCodecParameterString(),
      frameCount: stats.packetCount,
      durationMs,
      sampledFrames,
    }
  } finally {
    input.dispose()
  }
}

async function currentCanvas(fixture: Fixture): Promise<BenchmarkResult> {
  if (!fixture.prepared) return { status: 'missing-fixture', reason: 'NATIVE_FIXTURE_NOT_PREPARED' }
  const config = clipBrowserEncoderConfig(fixture.ratio)
  if (
    typeof VideoEncoder === 'undefined' ||
    !(await VideoEncoder.isConfigSupported(config.video)).supported
  )
    return { status: 'unsupported', reason: 'H264_ENCODER_UNAVAILABLE' }
  if (
    fixture.audio !== 'none' &&
    (typeof AudioEncoder === 'undefined' ||
      !(await AudioEncoder.isConfigSupported(config.audio)).supported)
  )
    return { status: 'unsupported', reason: 'AAC_ENCODER_UNAVAILABLE' }

  const started = performance.now()
  const phases: Record<string, number | null> = {
    sourceRead: null,
    nativeAssetPreparation: fixture.nativeAssetPreparationMs ?? null,
    serverSamplingWait: null,
    layoutAssets: null,
    video: null,
    audio: null,
    mux: null,
    upload: null,
  }
  let peakHeap = heapBytes()
  const sampler = setInterval(() => {
    const value = heapBytes()
    if (value !== null) peakHeap = Math.max(peakHeap ?? 0, value)
  }, 25)
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(new Error('BENCHMARK_TIMEOUT')), 180_000)
  let videoHandle: ReturnType<typeof renderBrowserVideo> | undefined
  const originals = new BrowserOriginals(
    [],
    async () => artifactURL(fixture.source.file),
    controller.signal,
    async (url, signal) => {
      const start = performance.now()
      try {
        const response = await fetch(url, { signal })
        if (!response.ok) throw new Error('BENCHMARK_SOURCE_UNAVAILABLE')
        return await response.blob()
      } finally {
        phases.sourceRead = (phases.sourceRead ?? 0) + performance.now() - start
      }
    },
  )
  const sheetBytes = new Map<string, Promise<Uint8Array<ArrayBuffer>>>()
  let videoMeasurements: unknown
  try {
    const assetStart = performance.now()
    const assets = fixture.assets.map(({ file, ...asset }) => ({
      ...asset,
      url: artifactURL(file),
    }))
    phases.layoutAssets = performance.now() - assetStart
    const videoStart = performance.now()
    videoHandle = renderBrowserVideo(
      { plan: fixture.plan, ratio: fixture.ratio, assets, collectMeasurements: true },
      [],
      async () => artifactURL(fixture.source.file),
      controller.signal,
      originals,
      async (instanceId, offset, signal) => {
        const page = fixture.sheets[instanceId]?.find(
          (value) => offset >= value.frameOffset && offset < value.frameOffset + value.cells,
        )
        if (!page) throw new Error('BENCHMARK_CAPTION_PAGE_UNAVAILABLE')
        let bytes = sheetBytes.get(page.file)
        if (!bytes) {
          bytes = fetch(artifactURL(page.file), { signal }).then(async (response) => {
            if (!response.ok) throw new Error('BENCHMARK_CAPTION_PAGE_UNAVAILABLE')
            return new Uint8Array(await response.arrayBuffer())
          })
          sheetBytes.set(page.file, bytes)
        }
        const { file, ...metadata } = page
        void file
        return { ...metadata, sheet: await bytes }
      },
    )
    const video = await videoHandle.result
    phases.video = performance.now() - videoStart
    videoMeasurements = video.measurements
    const audioStart = performance.now()
    const audio =
      fixture.audio === 'none'
        ? undefined
        : await renderBrowserAudio(fixture.plan, fixture.ratio, originals, controller.signal)
    if (fixture.audio !== 'none') phases.audio = performance.now() - audioStart
    const muxStart = performance.now()
    const blob = await muxMp4(video, audio, controller.signal)
    phases.mux = performance.now() - muxStart
    const elapsedMs = performance.now() - started
    const output = await verifyBenchmarkOutput(fixture, blob)
    return {
      status: 'done',
      elapsedMs,
      phases,
      videoMeasurements,
      peakResources: { mainThreadJSHeapBytes: peakHeap, workerHeapBytes: null, gpuBytes: null },
      output,
    }
  } catch (error) {
    if (error instanceof Error && 'measurements' in error) videoMeasurements = error.measurements
    return {
      status: 'failed',
      reason: error instanceof Error ? error.message : 'BENCHMARK_FAILED',
      elapsedMs: performance.now() - started,
      phases,
      videoMeasurements,
      peakResources: { mainThreadJSHeapBytes: peakHeap, workerHeapBytes: null, gpuBytes: null },
    }
  } finally {
    clearInterval(sampler)
    clearTimeout(timeout)
    controller.abort()
    videoHandle?.cancel()
    originals.dispose()
    sheetBytes.clear()
  }
}

adapters.set('canvas2d-current', { run: currentCanvas })
window.browserMediaBenchmark = {
  ready: true,
  environment,
  motion: previewMotion,
  verifyOutput: async (id, blob) => {
    const fixture = fixtures.cases.find((value) => value.id === id)
    if (!fixture) throw new Error('UNKNOWN_BENCHMARK_FIXTURE')
    return verifyBenchmarkOutput(fixture, blob)
  },
  register: (renderer, adapter) => {
    if (adapters.has(renderer)) throw new Error('Benchmark renderer already registered')
    adapters.set(renderer, adapter)
  },
  run: async (fixtureId, renderer) => {
    const fixture = fixtures.cases.find((value) => value.id === fixtureId)
    if (!fixture) return { status: 'missing-fixture', reason: 'UNKNOWN_FIXTURE' }
    const adapter = adapters.get(renderer)
    if (!adapter) return { status: 'not-implemented', reason: 'COMPARISON_ADAPTER_NOT_IMPLEMENTED' }
    return adapter.run(fixture)
  },
}
