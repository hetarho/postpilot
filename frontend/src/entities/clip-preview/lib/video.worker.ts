import { clipBrowserEncoderConfig } from '../model/browser-render-capability'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import type { BrowserVideoInput, EncodedClipChunk } from '../model/browser-video'
import type { VideoWorkerInput, VideoWorkerOutput } from '../model/video-worker-protocol'
import { compositeBrowserVideo } from '../model/composite-video'
import { RenderRasterCache } from '../model/render-raster-cache'
import type { CaptionCell } from '../model/caption-sheets'
import { MediaPhaseRecorder } from '@/shared/lib'
import { CLIP_VIDEO_MEASUREMENT_PHASES } from '../config/render-measurements'
import { readBrowserCompositionSnapshot } from '../model/browser-composition'
import { BrowserFootageResources } from '../model/browser-footage'
import type { BrowserMediaSourceAccess } from '@/shared/lib'

const send = (message: VideoWorkerOutput, transfer: Transferable[] = []) =>
  self.postMessage(message, { transfer })
let requestId = 0
const waiting = new Map<number, (bitmap: ImageBitmap) => void>()
const waitingCells = new Map<number, (cell: CaptionCell | undefined) => void>()
const waitingAccess = new Map<
  number,
  { resolve: (access: BrowserMediaSourceAccess) => void; reject: (error: unknown) => void }
>()
let activeController: AbortController | undefined

async function render(input: BrowserVideoInput, controller: AbortController) {
  if (input.snapshot)
    input = { ...input, snapshot: await readBrowserCompositionSnapshot(input.snapshot) }
  const measurements = input.collectMeasurements
    ? new MediaPhaseRecorder(CLIP_VIDEO_MEASUREMENT_PHASES)
    : undefined
  const signal = controller.signal
  signal.throwIfAborted()
  const footage = input.snapshot
    ? new BrowserFootageResources(
        input.snapshot,
        (sourceId, fingerprint, sourceSignal) =>
          new Promise((resolve, reject) => {
            const id = ++requestId
            const cleanup = () => {
              waitingAccess.delete(id)
              sourceSignal.removeEventListener('abort', cancelled)
            }
            const cancelled = () => {
              cleanup()
              reject(sourceSignal.reason)
            }
            sourceSignal.addEventListener('abort', cancelled, { once: true })
            waitingAccess.set(id, {
              resolve: (access) => {
                cleanup()
                resolve(access)
              },
              reject: (error) => {
                cleanup()
                reject(error)
              },
            })
            send({ type: 'sourceAccess', requestId: id, sourceId, fingerprint })
          }),
        signal,
        {
          opened: (elapsed) => measurements?.record('sourceOpen', elapsed),
          read: (_bytes, elapsed) => measurements?.record('sourceRead', elapsed),
          decoded: (elapsed) => measurements?.record('sourceDecode', elapsed),
        },
      )
    : undefined
  const config = clipBrowserEncoderConfig(input.ratio).video
  const canvas = new OffscreenCanvas(config.width, config.height)
  const context = canvas.getContext('2d', { alpha: false })
  if (!context) throw new Error('CLIP_CANVAS_UNAVAILABLE')
  const chunks: EncodedClipChunk[] = []
  let decoderConfig: VideoDecoderConfig | undefined
  let failure: DOMException | undefined
  let pressureReject: ((error: unknown) => void) | undefined
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
      pressureReject?.(error)
    },
  })
  const closeEncoder = () => {
    if (encoder.state !== 'closed') encoder.close()
  }
  signal.addEventListener('abort', closeEncoder, { once: true })
  const bitmaps = new RenderRasterCache(async (asset) => {
    const response = await fetch(asset.url, { signal })
    if (!response.ok) throw new Error('CLIP_PREVIEW_INVALID')
    return createImageBitmap(await response.blob())
  })
  try {
    encoder.configure(config)
    const timing = await compositeBrowserVideo(input, {
      context,
      measurements,
      footage,
      source: (fingerprint, timeMs) =>
        new Promise((resolve, reject) => {
          const id = ++requestId
          const cancelled = () => {
            waiting.delete(id)
            reject(signal.reason)
          }
          signal.addEventListener('abort', cancelled, { once: true })
          waiting.set(id, (bitmap) => {
            signal.removeEventListener('abort', cancelled)
            resolve(bitmap)
          })
          send({ type: 'source', requestId: id, fingerprint, timeMs })
        }),
      asset: (asset) => bitmaps.get(asset),
      // The server drew this caption's frame; the page fetches and cuts it out.
      captionFrame: (asset, frame) =>
        new Promise((resolve, reject) => {
          const id = ++requestId
          const cancelled = () => {
            waitingCells.delete(id)
            reject(signal.reason)
          }
          signal.addEventListener('abort', cancelled, { once: true })
          waitingCells.set(id, (cell) => {
            signal.removeEventListener('abort', cancelled)
            resolve(cell)
          })
          send({ type: 'frames', requestId: id, instanceId: asset.instanceId, frame })
        }),
      releaseAssets: (keys) => bitmaps.retain(keys),
      encode: async (timestamp, duration, keyFrame) => {
        signal.throwIfAborted()
        // A bounded batch also surfaces codec failures before requesting more source frames.
        if (encoder.encodeQueueSize >= CLIP_BROWSER_RENDER.encodeQueueFrames) {
          const wait = () =>
            new Promise<void>((resolve, reject) => {
              const cleanup = () => {
                encoder.removeEventListener('dequeue', dequeue)
                signal.removeEventListener('abort', cancelled)
                clearTimeout(timer)
                pressureReject = undefined
              }
              const failed = (error: unknown) => {
                cleanup()
                reject(error)
              }
              const dequeue = () => {
                if (failure) failed(failure)
                else if (encoder.encodeQueueSize < CLIP_BROWSER_RENDER.encodeQueueFrames) {
                  cleanup()
                  resolve()
                }
              }
              const cancelled = () => failed(signal.reason)
              const timer = setTimeout(
                () => failed(new Error('CLIP_VIDEO_ENCODER_TIMEOUT')),
                CLIP_BROWSER_RENDER.sourceTimeoutMs,
              )
              pressureReject = failed
              encoder.addEventListener('dequeue', dequeue)
              signal.addEventListener('abort', cancelled, { once: true })
              dequeue()
            })
          if (measurements) await measurements.measureAsync('encodeWait', wait)
          else await wait()
        }
        if (failure) throw failure
        const submitEnd = measurements?.begin('encodeSubmit')
        const frame = new VideoFrame(canvas, { timestamp, duration })
        try {
          encoder.encode(frame, { keyFrame })
        } finally {
          frame.close()
          submitEnd?.()
        }
      },
      progress: (progress) => {
        if (!signal.aborted && activeController === controller) send({ type: 'progress', progress })
      },
    })
    if (measurements) await measurements.measureAsync('encodeWait', () => encoder.flush())
    else await encoder.flush()
    if (failure) throw failure
    if (!decoderConfig || chunks.length !== timing.frameCount)
      throw new Error('CLIP_VIDEO_TRACK_INVALID')
    await footage?.dispose()
    signal.throwIfAborted()
    if (activeController !== controller) throw new DOMException('Superseded', 'AbortError')
    send(
      {
        type: 'done',
        track: {
          config,
          decoderConfig,
          chunks,
          ...timing,
          ...(measurements ? { measurements: measurements.snapshot() } : {}),
          ...(footage && input.collectMeasurements
            ? { sourceResources: footage.measurements() }
            : {}),
        },
      },
      chunks.map((chunk) => chunk.data.buffer),
    )
  } catch (error) {
    if (measurements && error instanceof Error)
      Object.assign(error, { measurements: measurements.snapshot() })
    throw error
  } finally {
    await footage?.dispose()
    signal.removeEventListener('abort', closeEncoder)
    if (encoder.state !== 'closed') encoder.close()
    bitmaps.retain(new Set())
  }
}

self.onmessage = (event: MessageEvent<VideoWorkerInput>) => {
  const message = event.data
  if (message.type === 'start') {
    activeController?.abort(new DOMException('Superseded', 'AbortError'))
    const controller = new AbortController()
    activeController = controller
    void render(message.input, controller).catch((error: unknown) => {
      if (activeController !== controller) return
      if (controller.signal.aborted) {
        send({ type: 'cancelled' })
        return
      }
      send({
        type: 'error',
        error: error instanceof Error ? error.message : String(error),
        ...(error instanceof Error && 'measurements' in error
          ? {
              measurements:
                error.measurements as import('../model/browser-video').BrowserVideoMeasurements,
            }
          : {}),
      })
    })
    return
  }
  if (message.type === 'cancel') {
    activeController?.abort(new DOMException('Cancelled', 'AbortError'))
    return
  }
  if (message.type === 'sourceAccess') {
    const pending = waitingAccess.get(message.requestId)
    if (!pending) return
    if (message.access) pending.resolve(message.access)
    else pending.reject(new Error(message.error ?? 'CLIP_SOURCE_UNAVAILABLE'))
    return
  }
  if (message.type === 'frames') {
    const cell = waitingCells.get(message.requestId)
    waitingCells.delete(message.requestId)
    const { bitmap, x, y, width, height } = message
    if (!cell) bitmap?.close()
    else cell(bitmap ? { bitmap, x, y, width, height } : undefined)
    return
  }
  const pending = waiting.get(message.requestId)
  waiting.delete(message.requestId)
  if (pending) pending(message.bitmap)
  else message.bitmap.close()
}
