import { clipBrowserEncoderConfig } from '../model/browser-render-capability'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import type { BrowserVideoInput, EncodedClipChunk } from '../model/browser-video'
import type { VideoWorkerInput, VideoWorkerOutput } from '../model/video-worker-protocol'
import { compositeBrowserVideo } from '../model/composite-video'
import { RenderRasterCache } from '../model/render-raster-cache'

const send = (message: VideoWorkerOutput, transfer: Transferable[] = []) =>
  self.postMessage(message, { transfer })
let requestId = 0
const waiting = new Map<number, (bitmap: ImageBitmap) => void>()

async function render(input: BrowserVideoInput) {
  const config = clipBrowserEncoderConfig(input.ratio).video
  const canvas = new OffscreenCanvas(config.width, config.height)
  const context = canvas.getContext('2d', { alpha: false })
  if (!context) throw new Error('CLIP_CANVAS_UNAVAILABLE')
  const chunks: EncodedClipChunk[] = []
  let decoderConfig: VideoDecoderConfig | undefined
  let failure: DOMException | undefined
  const encoder = new VideoEncoder({
    output: (chunk, metadata) => {
      const data = new Uint8Array(chunk.byteLength)
      chunk.copyTo(data)
      chunks.push({
        type: chunk.type,
        timestamp: chunk.timestamp,
        duration: chunk.duration ?? 0,
        data,
      })
      if (metadata?.decoderConfig) decoderConfig = metadata.decoderConfig
    },
    error: (error) => {
      failure = error
    },
  })
  const bitmaps = new RenderRasterCache(async (asset) => {
    const response = await fetch(asset.url)
    if (!response.ok) throw new Error('CLIP_PREVIEW_INVALID')
    return createImageBitmap(await response.blob())
  })
  try {
    encoder.configure(config)
    const timing = await compositeBrowserVideo(input, {
      context,
      source: (fingerprint, timeMs) =>
        new Promise((resolve) => {
          const id = ++requestId
          waiting.set(id, resolve)
          send({ type: 'source', requestId: id, fingerprint, timeMs })
        }),
      asset: (asset) => bitmaps.get(asset),
      releaseAssets: (keys) => bitmaps.retain(keys),
      encode: async (timestamp, duration, keyFrame) => {
        // A bounded batch also surfaces codec failures before requesting more source frames.
        if (encoder.encodeQueueSize >= CLIP_BROWSER_RENDER.encodeQueueFrames) await encoder.flush()
        if (failure) throw failure
        const frame = new VideoFrame(canvas, { timestamp, duration })
        try {
          encoder.encode(frame, { keyFrame })
        } finally {
          frame.close()
        }
      },
      progress: (progress) => send({ type: 'progress', progress }),
    })
    await encoder.flush()
    if (failure) throw failure
    if (!decoderConfig || chunks.length !== timing.frameCount)
      throw new Error('CLIP_VIDEO_TRACK_INVALID')
    send(
      { type: 'done', track: { config, decoderConfig, chunks, ...timing } },
      chunks.map((chunk) => chunk.data.buffer),
    )
  } finally {
    if (encoder.state !== 'closed') encoder.close()
    bitmaps.retain(new Set())
  }
}

self.onmessage = (event: MessageEvent<VideoWorkerInput>) => {
  const message = event.data
  if (message.type === 'start') {
    void render(message.input).catch((error: unknown) =>
      send({ type: 'error', error: error instanceof Error ? error.message : String(error) }),
    )
    return
  }
  const pending = waiting.get(message.requestId)
  waiting.delete(message.requestId)
  if (pending) pending(message.bitmap)
  else message.bitmap.close()
}
