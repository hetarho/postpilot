import {
  openOriginalVideo,
  OriginalVideoCursor,
  VideoFrameBudget,
  type BrowserMediaSourceAccess,
} from '@/shared/lib'
import { CLIP_VIDEO_DECODING } from '@/entities/clip-preview'

const decodedFrames = new Set<VideoFrame>()
let peakDecodedFrames = 0
const nativeClose = VideoFrame.prototype.close
VideoFrame.prototype.close = function () {
  decodedFrames.delete(this)
  return nativeClose.call(this)
}
const NativeDecoder = VideoDecoder
self.VideoDecoder = class extends NativeDecoder {
  constructor(options: VideoDecoderInit) {
    super({
      ...options,
      output: (frame) => {
        decodedFrames.add(frame)
        peakDecodedFrames = Math.max(peakDecodedFrames, decodedFrames.size)
        options.output(frame)
      },
    })
  }
}

/** Real codec fixture, separate from normal composition; pixel readback is diagnostic-only. */
self.onmessage = async (
  event: MessageEvent<{
    access: BrowserMediaSourceAccess
    id: string
    startUs?: number
    ratePermille?: number
  }>,
) => {
  const controller = new AbortController(),
    { access, id } = event.data
  let input: Awaited<ReturnType<typeof openOriginalVideo>> | undefined,
    cursor: OriginalVideoCursor | undefined
  let readBytes = 0,
    readCalls = 0
  const budget = new VideoFrameBudget(2, CLIP_VIDEO_DECODING.presentationBytes)
  try {
    input = await openOriginalVideo(access, CLIP_VIDEO_DECODING, controller.signal, {
      read: (bytes) => {
        readBytes += bytes
        readCalls++
      },
    })
    const startUs = event.data.startUs ?? 2_100_000
    cursor = new OriginalVideoCursor(
      input,
      {
        startUs,
        endUs: startUs + 1_000_000,
        ratePermille: event.data.ratePermille ?? 1000,
        fps: 30,
      },
      budget,
      controller.signal,
    )
    const selected: number[] = []
    const canvas = new OffscreenCanvas(1080, 1920),
      context = canvas.getContext('2d')!
    for (let n = 0; n < 8; n++) {
      const resource = await cursor.frame(n)
      try {
        selected.push(resource.timestampUs)
        resource.draw(context, { x: 0, y: 0, width: 1080, height: 1920 })
      } finally {
        resource.close()
      }
    }
    await cursor.close()
    input.dispose()
    await new Promise((resolve) => setTimeout(resolve, 50))
    self.postMessage({
      id,
      selected,
      metadata: input.metadata,
      readBytes,
      readCalls,
      budget: budget.snapshot(),
      decodedResources: { peakDecodedFrames, liveDecodedFrames: decodedFrames.size },
      qualified: false,
    })
  } catch (error) {
    input?.dispose()
    await cursor?.close()
    await new Promise((resolve) => setTimeout(resolve, 50))
    self.postMessage({
      id,
      error: error instanceof Error ? error.message : String(error),
      readBytes,
      readCalls,
      budget: budget.snapshot(),
      decodedResources: { peakDecodedFrames, liveDecodedFrames: decodedFrames.size },
      qualified: false,
    })
  }
}
