import { clipBrowserEncoderConfig } from './browser-render-capability'
import { previewCrop, previewFrame, previewMotion } from './draft-preview'
import { timelineCuts } from './edit-plan'
import type { PreparedAsset } from './preview-assets'
import { CLIP_BROWSER_RENDER } from '../config'
import type { BrowserVideoInput, BrowserVideoProgress } from './browser-video'

interface CompositePorts {
  context: Pick<
    OffscreenCanvasRenderingContext2D,
    'globalAlpha' | 'fillStyle' | 'fillRect' | 'drawImage'
  >
  source: (fingerprint: string, timeMs: number) => Promise<ImageBitmap>
  asset: (asset: PreparedAsset) => Promise<ImageBitmap>
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
      const bitmap = await ports.asset(asset)
      const motion = asset.representativeFrame
        ? previewMotion(asset, timeMs)
        : { opacity: 1, dy: 0 }
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
