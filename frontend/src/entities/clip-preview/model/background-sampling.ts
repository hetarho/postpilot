import { CLIP_DESIGN, CLIP_TRANSITION } from '@/entities/clip-design/@x/clip-preview'
import { BrowserFootageResources, type BrowserSourceAccess } from './browser-footage'
import {
  BrowserSnapshotEpoch,
  evaluateBrowserFrame,
  type BrowserCompositionSnapshot,
} from './browser-composition'
import { BrowserLocalComponents, type BrowserBackgroundGeometry } from './local-components'
import { ClipInkError } from './ink-typography'
import { NATIVE_SOURCE_COLOR_VERSION } from '@/shared/lib'
import { backgroundContrastNotices } from './background-paint'
import {
  backgroundSampleFrames,
  browserGroundDecisions,
  integerBackgroundRegion,
  meanRegionPixels,
  nativeFadeBlackPixel,
  summarizeBrowserGround,
  CLIP_BACKGROUND_VERSION,
  type BackgroundRGB,
  type BrowserGround,
} from './background-math'

export const CLIP_BACKGROUND_LIMITS = {
  maxReads: 2400,
  maxRegionBytes: 8 * 1024 * 1024,
  maxLivePixelBytes: 24 * 1024 * 1024,
} as const
export interface BrowserBackgroundMeasurement {
  instanceId: string
  elementId: string
  cutId: string
  phraseIndex: number
  sampleFrames: [number, number, number]
  geometry: BrowserBackgroundGeometry
  ground: BrowserGround
  scrim: boolean
  accentWhite: boolean
}
export interface BrowserBackgroundEvidence {
  version: string
  snapshotFingerprint: string
  sourceColorVersion: string
  digest: string
  measurements: BrowserBackgroundMeasurement[]
  notices: { code: string; action: string; elementId: string; cutId: string }[]
}
type Read = { visual: number; ordinal: number; frame: number }
export interface BrowserBackgroundDiagnostics {
  resources: ReturnType<BrowserFootageResources['measurements']>
  roiReads: number
  peakRegionBytes: number
}

/** Original-only clip sampler. Sorted frame requests share one decoded active
 * cut set; only cropped region pixels are read back and released each time. */
export async function measureBrowserBackground(
  snapshot: BrowserCompositionSnapshot,
  components: BrowserLocalComponents,
  access: BrowserSourceAccess,
  signal: AbortSignal,
  observed?: (diagnostics: BrowserBackgroundDiagnostics) => void,
): Promise<BrowserBackgroundEvidence> {
  signal.throwIfAborted()
  if (typeof OffscreenCanvas === 'undefined') throw new ClipInkError('CLIP_BACKGROUND_UNSUPPORTED')
  const epoch = new BrowserSnapshotEpoch(),
    token = epoch.begin(snapshot)
  const check = () => {
    signal.throwIfAborted()
    epoch.assertCurrent(token)
  }
  const footage = new BrowserFootageResources(snapshot, access, signal)
  const visuals: BrowserBackgroundMeasurement[] = [],
    reads: Read[] = []
  const colors: BackgroundRGB[][] = []
  const canvasSize = CLIP_DESIGN.ratios[snapshot.ratio].canvas
  let pixelBytes = 0
  let roiReads = 0
  try {
    for (const component of snapshot.components) {
      check()
      const frame = evaluateBrowserFrame(snapshot, component.visibleFirstFrame)
      const state = frame.components.find((s) => s.component === component)
      if (!state) throw new ClipInkError('CLIP_BACKGROUND_MISSING', component.instanceId)
      const geometry = await components.backgroundGeometry(state, signal)
      check()
      if (!geometry || geometry.plate) continue
      const sampleFrames = backgroundSampleFrames(
        component.startMs,
        component.endMs,
        snapshot.frameCount,
        30,
      )
      const visual = visuals.length
      visuals.push({
        instanceId: component.instanceId,
        elementId: component.element.elementId,
        cutId: component.element.cutId,
        phraseIndex: component.phraseIndex ?? 0,
        sampleFrames,
        geometry,
        ground: { frames: [], mean: 0, sigma: 0, rgb: [0, 0, 0] },
        scrim: false,
        accentWhite: false,
      })
      colors.push([])
      for (let ordinal = 0; ordinal < 3; ordinal++)
        reads.push({ visual, ordinal, frame: sampleFrames[ordinal]! })
      if (reads.length > CLIP_BACKGROUND_LIMITS.maxReads)
        throw new ClipInkError('CLIP_BACKGROUND_MEMORY_LIMIT')
    }
    reads.sort((a, b) => a.frame - b.frame || a.visual - b.visual || a.ordinal - b.ordinal)
    for (let start = 0; start < reads.length;) {
      const frame = reads[start]!.frame
      let end = start + 1
      while (end < reads.length && reads[end]!.frame === frame) end++
      check()
      const evaluated = evaluateBrowserFrame(snapshot, frame)
      const prepared = await footage.prepare(evaluated)
      try {
        check()
        const cache = new Map<string, BackgroundRGB>()
        for (const read of reads.slice(start, end)) {
          check()
          const rect = integerBackgroundRegion(
            visuals[read.visual]!.geometry.region,
            canvasSize.width,
            canvasSize.height,
          )
          const key = JSON.stringify(rect)
          let mean = cache.get(key)
          if (!mean) {
            const bytes = rect.width * rect.height * 4
            if (
              bytes > CLIP_BACKGROUND_LIMITS.maxRegionBytes ||
              bytes * prepared.length > CLIP_BACKGROUND_LIMITS.maxLivePixelBytes
            )
              throw new ClipInkError('CLIP_BACKGROUND_MEMORY_LIMIT')
            const pixels = prepared.map(({ resource, layer }) => {
              const canvas = new OffscreenCanvas(rect.width, rect.height),
                context = canvas.getContext('2d', { willReadFrequently: true })
              try {
                if (!context) throw new ClipInkError('CLIP_BACKGROUND_UNSUPPORTED')
                context.setTransform(1, 0, 0, 1, -rect.x, -rect.y)
                resource.draw(context, {
                  x: (layer.crop.left * canvasSize.width) / 100,
                  y: (layer.crop.top * canvasSize.height) / 100,
                  width: (layer.crop.width * canvasSize.width) / 100,
                  height: (layer.crop.height * canvasSize.height) / 100,
                })
                return context.getImageData(0, 0, rect.width, rect.height).data
              } finally {
                canvas.width = 0
                canvas.height = 0
              }
            })
            pixelBytes = Math.max(pixelBytes, bytes * pixels.length)
            roiReads += pixels.length
            if (prepared.length === 1) mean = meanRegionPixels(pixels[0]!, rect.width, rect.height)
            else {
              const weights = prepared.map((p) => p.layer.weight)
              const incoming = snapshot.plan.cuts.find(
                (c) => c.id === prepared[1]!.layer.cutInstanceId,
              )
              if (incoming?.transitionMs === CLIP_TRANSITION.black_ms) {
                const sum: [number, number, number] = [0, 0, 0]
                let n = 0
                for (let y = 0; y < rect.height; y += 2)
                  for (let x = 0; x < rect.width; x += 2) {
                    const at = (y * rect.width + x) * 4
                    const a = pixels[0]!,
                      b = pixels[1]!
                    const rgb = nativeFadeBlackPixel(
                      [a[at]! / 255, a[at + 1]! / 255, a[at + 2]! / 255],
                      [b[at]! / 255, b[at + 1]! / 255, b[at + 2]! / 255],
                      weights[0]!,
                      weights[1]!,
                    )
                    for (let c = 0; c < 3; c++) sum[c as 0 | 1 | 2] += rgb[c as 0 | 1 | 2]
                    n++
                  }
                mean = sum.map((v) => v / n) as [number, number, number]
              } else {
                const a = meanRegionPixels(pixels[0]!, rect.width, rect.height),
                  b = meanRegionPixels(pixels[1]!, rect.width, rect.height)
                mean = a.map((v, c) => v * weights[0]! + b[c as 0 | 1 | 2] * weights[1]!) as [
                  number,
                  number,
                  number,
                ]
              }
            }
            cache.set(key, mean)
          }
          colors[read.visual]![read.ordinal] = mean
        }
      } finally {
        prepared.forEach((p) => p.resource.close())
      }
      start = end
    }
    check()
    for (let i = 0; i < visuals.length; i++) {
      const ground = summarizeBrowserGround(colors[i]!)
      Object.assign(visuals[i]!, { ground, ...browserGroundDecisions(ground) })
    }
    if (
      footage
        .measurements()
        .originals.some((source) => source.colorVersion !== NATIVE_SOURCE_COLOR_VERSION)
    )
      throw new ClipInkError('CLIP_SOURCE_COLOR_UNSUPPORTED')
    const notices = [
      ...new Map(
        visuals
          .flatMap((v) => backgroundContrastNotices(snapshot.ratio, v))
          .map((n) => [JSON.stringify(n), n]),
      ).values(),
    ]
    const proof = {
      version: CLIP_BACKGROUND_VERSION,
      snapshotFingerprint: snapshot.authoritativeFingerprint ?? snapshot.snapshotFingerprint,
      sourceColorVersion: NATIVE_SOURCE_COLOR_VERSION,
      measurements: visuals,
      notices,
    }
    const data = new TextEncoder().encode(JSON.stringify(proof))
    const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', data))
    check()
    return { ...proof, digest: [...hash].map((v) => v.toString(16).padStart(2, '0')).join('') }
  } finally {
    epoch.cancel()
    await footage.dispose()
    observed?.({ resources: footage.measurements(), roiReads, peakRegionBytes: pixelBytes })
  }
}
