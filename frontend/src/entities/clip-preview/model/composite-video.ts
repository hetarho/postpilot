import { clipBrowserEncoderConfig } from './browser-render-capability'
import { previewCrop, previewFrame, previewMotion } from './draft-preview'
import { timelineCuts } from '@/entities/clip-plan/@x/clip-preview'
import type { PreparedAsset } from './preview-assets'
import type { CaptionCell } from './caption-sheets'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import type { BrowserVideoInput, BrowserVideoProgress } from './browser-video'

interface CompositePorts {
  context: Pick<
    OffscreenCanvasRenderingContext2D,
    'globalAlpha' | 'fillStyle' | 'fillRect' | 'drawImage'
  >
  source: (fingerprint: string, timeMs: number) => Promise<ImageBitmap>
  asset: (asset: PreparedAsset) => Promise<ImageBitmap>
  /** One frame of a sequence-rendered caption, drawn by the server (CLIP-159). */
  captionFrame: (asset: PreparedAsset, frame: number) => Promise<CaptionCell | undefined>
  releaseAssets: (keys: Set<string>) => void
  encode: (timestamp: number, duration: number, keyFrame: boolean) => Promise<void>
  progress: (value: BrowserVideoProgress) => void
}

/** Both the preview and export read the plan's resolved intervals and manifest motion. */
export async function compositeBrowserVideo(input: BrowserVideoInput, ports: CompositePorts) {
  const config = clipBrowserEncoderConfig(input.ratio).video
  const timeline = timelineCuts(input.plan)
  const durationMs = timeline.at(-1)?.endMs ?? 0
  const fps = CLIP_BROWSER_RENDER.frameRate
  const totalFrames = Math.round((durationMs * fps) / 1000)
  if (!totalFrames) throw new Error('CLIP_RENDER_EMPTY')
  const assets = [...input.assets].sort((a, b) => a.layer - b.layer)
  const ctx = ports.context
  for (let frame = 0; frame < totalFrames; frame++) {
    const timeMs = (frame * 1000) / fps
    ctx.globalAlpha = 1
    ctx.fillStyle = CLIP_BROWSER_RENDER.matte
    ctx.fillRect(0, 0, config.width, config.height)
    for (const current of previewFrame(timeline, timeMs)) {
      const bitmap = await ports.source(current.cut.fingerprint, current.sourceMs)
      try {
        const crop = previewCrop(
          bitmap.width,
          bitmap.height,
          config.width,
          config.height,
          current.cut.focal,
        )
        ctx.globalAlpha = current.opacity
        ctx.drawImage(
          bitmap,
          (crop.left * config.width) / 100,
          (crop.top * config.height) / 100,
          (crop.width * config.width) / 100,
          (crop.height * config.height) / 100,
        )
      } finally {
        bitmap.close()
      }
    }
    const active = assets.filter((asset) => timeMs >= asset.startMs && timeMs < asset.endMs)
    ports.releaseAssets(new Set(active.map((asset) => asset.key)))
    for (const asset of active) {
      // A sequence-rendered style is drawn by the server for THIS output frame,
      // motion and all, so nothing here adds to it: the browser owns only which
      // frame belongs where (CLIP-159).
      if (asset.representativeFrame) {
        const cell = await ports.captionFrame(asset, frame)
        if (!cell) continue
        try {
          ctx.globalAlpha = 1
          ctx.drawImage(cell.bitmap, cell.x, cell.y, cell.width, cell.height)
        } finally {
          cell.bitmap.close()
        }
        continue
      }
      const bitmap = await ports.asset(asset)
      // A static style is one raster moved the way that style declares (CDS-4):
      // the server chain fades and settles it from these very in/out/dy numbers,
      // so a browser render that held it still delivered a different clip from
      // the same plan (CLIP-157). A rapid phrase carries 0/0/0 and is held by
      // the same call.
      const motion = previewMotion(asset, timeMs)
      ctx.globalAlpha = motion.opacity
      ctx.drawImage(bitmap, asset.x, asset.y + motion.dy, asset.width, asset.height)
    }
    const timestamp = Math.round((frame * 1_000_000) / fps)
    const duration = Math.round(((frame + 1) * 1_000_000) / fps) - timestamp
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
