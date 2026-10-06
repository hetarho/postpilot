import { clipBrowserEncoderConfig } from './browser-render-capability'
import { frameLayers, frameTimeline, previewCrop, previewMotion } from './draft-preview'
import type { PreparedAsset } from './preview-assets'
import type { CaptionCell } from './caption-sheets'
import { CLIP_BROWSER_RENDER, CLIP_TRANSITION } from '@/entities/clip-design/@x/clip-preview'
import type { BrowserVideoInput, BrowserVideoProgress } from './browser-video'
import type { MediaPhaseRecorder } from '@/shared/lib'
import type { CLIP_VIDEO_MEASUREMENT_PHASES } from '../config/render-measurements'
import { evaluateBrowserFrame } from './browser-composition'
import type { BrowserFootageResources } from './browser-footage'

export interface CompositeFramePorts {
  assertCurrent?: () => void
  displayed?: (frames: readonly { cutId: string; sourceMs: number; outputMs: number }[]) => void
  context: Pick<
    OffscreenCanvasRenderingContext2D,
    'globalAlpha' | 'fillStyle' | 'fillRect' | 'drawImage'
  >
  source?: (fingerprint: string, timeMs: number) => Promise<ImageBitmap>
  footage?: BrowserFootageResources
  nativeFadeBlack?: (rest: number) => void
  local?: (frame: ReturnType<typeof evaluateBrowserFrame>) => Promise<void>
  asset: (asset: PreparedAsset) => Promise<ImageBitmap>
  /** One frame of a sequence-rendered caption, drawn by the server (CLIP-159). */
  captionFrame: (asset: PreparedAsset, frame: number) => Promise<CaptionCell | undefined>
  releaseAssets: (keys: Set<string>) => void
  measurements?: MediaPhaseRecorder<(typeof CLIP_VIDEO_MEASUREMENT_PHASES)[number]>
}

interface CompositePorts extends CompositeFramePorts {
  encode: (timestamp: number, duration: number, keyFrame: boolean) => Promise<void>
  progress: (value: BrowserVideoProgress) => void
}

/** Both the preview and export read the plan's resolved intervals and manifest motion. The
 *  footage of each output frame is the server render's: the same cuts on the same frames,
 *  joined by the same xfade curve (CLIP-192). */
export async function compositeBrowserFrame(
  input: BrowserVideoInput,
  ports: CompositeFramePorts,
  frame: number,
) {
  const config = clipBrowserEncoderConfig(input.ratio).video
  const fps = CLIP_BROWSER_RENDER.frameRate
  const timeline = frameTimeline(input.plan, fps)
  const totalFrames = input.snapshot?.frameCount ?? timeline.total
  if (!Number.isSafeInteger(frame) || frame < 0 || frame >= totalFrames)
    throw new Error('CLIP_SNAPSHOT_INVALID')
  if (
    input.snapshot &&
    (input.snapshot.ratio !== input.ratio ||
      JSON.stringify(input.snapshot.plan) !== JSON.stringify(input.plan))
  )
    throw new Error('CLIP_SNAPSHOT_SUPERSEDED')
  const assets = [...input.assets].sort((a, b) => a.layer - b.layer)
  const ctx = ports.context
  const measurements = ports.measurements
  ports.assertCurrent?.()
  const displayed: { cutId: string; sourceMs: number; outputMs: number }[] = []
  const timeMs = (frame * 1000) / fps
  const matteEnd = measurements?.begin('composeSubmit')
  ctx.globalAlpha = 1
  ctx.fillStyle = CLIP_BROWSER_RENDER.matte
  ctx.fillRect(0, 0, config.width, config.height)
  matteEnd?.()
  const evaluated = input.snapshot ? evaluateBrowserFrame(input.snapshot, frame) : undefined
  if (evaluated && ports.footage) {
    const prepared = measurements
      ? await measurements.measureAsync('sourceWait', () => ports.footage!.prepare(evaluated))
      : await ports.footage.prepare(evaluated)
    const drawEnd = measurements?.begin('composeSubmit')
    try {
      ports.assertCurrent?.()
      for (const { layer, resource } of prepared) {
        displayed.push({
          cutId: layer.cutInstanceId,
          sourceMs: resource.timestampUs / 1000,
          outputMs: timeMs,
        })
        ctx.globalAlpha = layer.alpha
        resource.draw(ctx as OffscreenCanvasRenderingContext2D, {
          x: (layer.crop.left * config.width) / 100,
          y: (layer.crop.top * config.height) / 100,
          width: (layer.crop.width * config.width) / 100,
          height: (layer.crop.height * config.height) / 100,
        })
      }
    } finally {
      for (const { resource } of prepared) resource.close()
      drawEnd?.()
    }
  } else {
    const layers =
      evaluated?.footageLayers.map((layer) => ({
        cut: input.plan.cuts.find((cut) => cut.id === layer.cutInstanceId)!,
        sourceMs: layer.sourceTimestampUs / 1000,
        alpha: layer.alpha,
      })) ?? frameLayers(timeline, frame)
    for (const current of layers) {
      if (!ports.source) throw new Error('CLIP_SOURCE_UNAVAILABLE')
      const bitmap = measurements
        ? await measurements.measureAsync('sourceWait', () =>
            ports.source!(current.cut.fingerprint, current.sourceMs),
          )
        : await ports.source(current.cut.fingerprint, current.sourceMs)
      const drawEnd = measurements?.begin('composeSubmit')
      try {
        ports.assertCurrent?.()
        const crop = previewCrop(
          bitmap.width,
          bitmap.height,
          config.width,
          config.height,
          current.cut.focal,
        )
        ctx.globalAlpha = current.alpha
        ctx.drawImage(
          bitmap,
          (crop.left * config.width) / 100,
          (crop.top * config.height) / 100,
          (crop.width * config.width) / 100,
          (crop.height * config.height) / 100,
        )
      } finally {
        drawEnd?.()
        bitmap.close()
      }
    }
  }
  if (evaluated && evaluated.footageLayers.length === 2) {
    const incoming = input.plan.cuts.find((c) => c.id === evaluated.footageLayers[1]!.cutInstanceId)
    if (incoming?.transitionMs === CLIP_TRANSITION.black_ms) {
      if (!ports.nativeFadeBlack) throw new Error('CLIP_FADEBLACK_COMPOSITOR_UNSUPPORTED')
      ports.nativeFadeBlack(
        Math.max(0, 1 - evaluated.footageLayers.reduce((n, l) => n + l.weight, 0)),
      )
    }
  }
  const active = assets.filter((asset) => timeMs >= asset.startMs && timeMs < asset.endMs)
  ports.releaseAssets(new Set(active.map((asset) => asset.key)))
  for (const asset of active) {
    // A sequence-rendered style is drawn by the server for THIS output frame,
    // motion and all, so nothing here adds to it: the browser owns only which
    // frame belongs where (CLIP-159).
    if (asset.representativeFrame) {
      const cell = measurements
        ? await measurements.measureAsync('assetWait', () => ports.captionFrame(asset, frame))
        : await ports.captionFrame(asset, frame)
      if (!cell) continue
      const cellEnd = measurements?.begin('composeSubmit')
      try {
        ctx.globalAlpha = 1
        ctx.drawImage(cell.bitmap, cell.x, cell.y, cell.width, cell.height)
      } finally {
        cellEnd?.()
        cell.bitmap.close()
      }
      continue
    }
    const bitmap = measurements
      ? await measurements.measureAsync('assetWait', () => ports.asset(asset))
      : await ports.asset(asset)
    const assetEnd = measurements?.begin('composeSubmit')
    // A static style is one raster moved the way that style declares (CDS-4):
    // the server chain fades and settles it from these very in/out/dy numbers,
    // so a browser render that held it still delivered a different clip from
    // the same plan (CLIP-157). A rapid phrase carries 0/0/0 and is held by
    // the same call.
    const motion = previewMotion(asset, timeMs)
    ctx.globalAlpha = motion.opacity
    ctx.drawImage(bitmap, asset.x, asset.y + motion.dy, asset.width, asset.height)
    assetEnd?.()
  }
  if (evaluated && ports.local) await ports.local(evaluated)
  ports.assertCurrent?.()
  ports.displayed?.(displayed)
  return evaluated
}

export async function compositeBrowserVideo(input: BrowserVideoInput, ports: CompositePorts) {
  if (
    input.snapshot &&
    (input.snapshot.ratio !== input.ratio ||
      JSON.stringify(input.snapshot.plan) !== JSON.stringify(input.plan))
  )
    throw new Error('CLIP_SNAPSHOT_SUPERSEDED')
  const fps = CLIP_BROWSER_RENDER.frameRate
  const timeline = frameTimeline(input.plan, fps)
  const totalFrames = input.snapshot?.frameCount ?? timeline.total
  if (!totalFrames) throw new Error('CLIP_RENDER_EMPTY')
  for (let frame = 0; frame < totalFrames; frame++) {
    const evaluated = await compositeBrowserFrame(input, ports, frame)
    const timestamp = evaluated?.timestampUs ?? Math.round((frame * 1_000_000) / fps)
    const duration =
      evaluated?.durationUs ?? Math.round(((frame + 1) * 1_000_000) / fps) - timestamp
    await ports.encode(
      timestamp,
      duration,
      frame % CLIP_BROWSER_RENDER.keyFrameIntervalFrames === 0,
    )
    ports.progress({ completedFrames: frame + 1, totalFrames })
  }
  ports.releaseAssets(new Set())
  return { frameCount: totalFrames, durationUs: Math.round((totalFrames * 1_000_000) / fps) }
}
