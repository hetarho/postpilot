import { Input, MP4, QTFF, WEBM, MATROSKA, VideoSampleSink, type VideoSample } from 'mediabunny'
import {
  createFiniteMediaSource,
  MediaRangeError,
  type BrowserMediaSourceAccess,
  type MediaRangeLimits,
  type MediaRangePorts,
} from './range-source'
import { VideoFrameBudget } from './frame-budget'

export interface OriginalVideoMetadata {
  provenance: 'browser_client'
  codec: string
  codedWidth: number
  codedHeight: number
  displayWidth: number
  displayHeight: number
  timeResolution: number
  firstTimestamp: number
  durationFromMetadata: number | null
  rotation: number
  flip: boolean
}
export interface VideoRangeSample {
  timestamp: number
  duration: number
  displayWidth: number
  displayHeight: number
  rotation: number
  flip: boolean
  toVideoFrame(): VideoFrame
  close(): void
}
export interface OriginalVideoInput {
  metadata: OriginalVideoMetadata
  samples(start: number, end: number): AsyncIterableIterator<VideoRangeSample>
  dispose(): void
}
export interface OriginalVideoPorts extends MediaRangePorts {
  decode?: (elapsedMs: number) => void
}
export async function openOriginalVideo(
  access: BrowserMediaSourceAccess,
  limits: MediaRangeLimits,
  signal: AbortSignal,
  ports: OriginalVideoPorts = {},
): Promise<OriginalVideoInput> {
  const reader = createFiniteMediaSource(access, limits, signal, ports)
  // Only finite video containers; a playlist never gains authority to fetch
  // other URLs through a browser render's owner-bound source access.
  const input = new Input({ source: reader.source, formats: [MP4, QTFF, WEBM, MATROSKA] })
  const abort = () => input.dispose()
  signal.addEventListener('abort', abort, { once: true })
  const dispose = () => {
    signal.removeEventListener('abort', abort)
    input.dispose()
    reader.dispose()
  }
  try {
    signal.throwIfAborted()
    const track = await input.getPrimaryVideoTrack()
    if (!track || !(await track.canDecode()))
      throw new MediaRangeError('CLIP_SOURCE_CODEC_UNSUPPORTED')
    if (await track.isLive()) throw new MediaRangeError('CLIP_SOURCE_RANGE_UNSUPPORTED')
    const [
      codedWidth,
      codedHeight,
      displayWidth,
      displayHeight,
      timeResolution,
      firstTimestamp,
      durationFromMetadata,
      rotation,
      flip,
      codec,
    ] = await Promise.all([
      track.getCodedWidth(),
      track.getCodedHeight(),
      track.getDisplayWidth(),
      track.getDisplayHeight(),
      track.getTimeResolution(),
      track.getFirstTimestamp(),
      track.getDurationFromMetadata(),
      track.getRotation(),
      track.getFlip(),
      track.getCodec(),
    ])
    signal.throwIfAborted()
    const sink = new VideoSampleSink(track)
    const metadata: OriginalVideoMetadata = {
      provenance: 'browser_client',
      codedWidth,
      codedHeight,
      displayWidth,
      displayHeight,
      timeResolution,
      firstTimestamp,
      durationFromMetadata,
      rotation,
      flip,
      codec: codec ?? 'unknown',
    }
    // Time resolution is a PTS lattice, not proof of original minimum cadence.
    // Caller-authorized allowedRatePermille remains the slow-motion authority.
    return {
      metadata,
      samples: (start, end) => sink.samples(start, end) as AsyncIterableIterator<VideoSample>,
      dispose,
    }
  } catch (error) {
    dispose()
    throw error
  }
}

/** Native fps round=near, using integer original PTS ticks rather than rounded
 * millisecond requests. Ties round away from zero for these nonnegative spans. */
export function nativeOutputFrame(
  sampleTicks: number,
  originTicks: number,
  timeResolution: number,
  ratePermille: number,
  fps: number,
): number {
  for (const n of [sampleTicks, originTicks, timeResolution, ratePermille, fps])
    if (!Number.isSafeInteger(n)) throw new MediaRangeError('CLIP_SOURCE_TIMESTAMP_INVALID')
  if (sampleTicks < originTicks || timeResolution <= 0 || ratePermille <= 0 || fps <= 0)
    throw new MediaRangeError('CLIP_SOURCE_TIMESTAMP_INVALID')
  const numerator = BigInt(sampleTicks - originTicks) * BigInt(fps) * 1000n
  const denominator = BigInt(timeResolution) * BigInt(ratePermille)
  return Number((2n * numerator + denominator) / (2n * denominator))
}

export interface VideoDrawRect {
  x: number
  y: number
  width: number
  height: number
}
export interface DecodedVideoResource {
  /** A presentation lease. Closing this frame directly does not release its budget. */
  readonly frame: VideoFrame
  readonly timestampUs: number
  readonly width: number
  readonly height: number
  draw(
    context: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
    destination: VideoDrawRect,
  ): void
  close(): void
}

export class OriginalVideoCursor {
  private iterator: AsyncIterableIterator<VideoRangeSample>
  private current?: VideoRangeSample
  private next?: VideoRangeSample
  private initialized = false
  private originTicks?: number
  private lastFrame = -1
  private closed = false
  private leases = new Set<DecodedVideoResource>()
  private frameExports = 0
  constructor(
    private input: OriginalVideoInput,
    private range: { startUs: number; endUs: number; ratePermille: number; fps: number },
    private budget: VideoFrameBudget,
    private signal: AbortSignal,
    private ports: OriginalVideoPorts = {},
  ) {
    if (
      !Number.isSafeInteger(range.startUs) ||
      !Number.isSafeInteger(range.endUs) ||
      range.startUs < 0 ||
      range.endUs <= range.startUs
    )
      throw new MediaRangeError('CLIP_SOURCE_TIMESTAMP_INVALID')
    this.iterator = input.samples(range.startUs / 1_000_000, range.endUs / 1_000_000)
  }
  private ticks(sample: VideoRangeSample) {
    return Math.round(sample.timestamp * this.input.metadata.timeResolution)
  }
  private inRange(sample: VideoRangeSample) {
    const tick = BigInt(this.ticks(sample)) * 1_000_000n
    const resolution = BigInt(this.input.metadata.timeResolution)
    return (
      tick >= BigInt(this.range.startUs) * resolution &&
      tick < BigInt(this.range.endUs) * resolution
    )
  }
  private async readNext() {
    this.signal.throwIfAborted()
    const began = performance.now()
    for (;;) {
      const item = await this.iterator.next()
      if (this.closed || this.signal.aborted) {
        item.value?.close()
        this.signal.throwIfAborted()
        throw new MediaRangeError('CLIP_SOURCE_SUPERSEDED')
      }
      if (item.done) {
        this.ports.decode?.(performance.now() - began)
        return undefined
      }
      if (this.inRange(item.value)) {
        this.ports.decode?.(performance.now() - began)
        return item.value
      }
      item.value.close() // Accurate preceding-keyframe decode never displays guard samples.
    }
  }
  async frame(localFrame: number): Promise<DecodedVideoResource> {
    this.signal.throwIfAborted()
    if (
      this.closed ||
      !Number.isSafeInteger(localFrame) ||
      localFrame < this.lastFrame ||
      localFrame < 0
    )
      throw new MediaRangeError('CLIP_SOURCE_SUPERSEDED')
    this.lastFrame = localFrame
    if (!this.initialized) {
      this.next = await this.readNext()
      this.originTicks = this.next ? this.ticks(this.next) : undefined
      this.initialized = true
    }
    while (
      this.next &&
      nativeOutputFrame(
        this.ticks(this.next),
        this.originTicks!,
        this.input.metadata.timeResolution,
        this.range.ratePermille,
        this.range.fps,
      ) <= localFrame
    ) {
      this.current?.close()
      this.current = this.next
      this.next = undefined
      this.next = await this.readNext()
    }
    if (!this.current) throw new MediaRangeError('CLIP_SOURCE_FRAME_MISSING')
    const sample = this.current
    const reserved = this.input.metadata.codedWidth * this.input.metadata.codedHeight * 4
    const release = await this.budget.acquire(reserved, this.signal)
    let frame: VideoFrame | undefined
    try {
      this.signal.throwIfAborted()
      if (this.closed || this.current !== sample)
        throw new MediaRangeError('CLIP_SOURCE_SUPERSEDED')
      frame = sample.toVideoFrame()
      this.frameExports++
      if (frame.allocationSize() > reserved) throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
      let released = false
      const resource: DecodedVideoResource = {
        frame,
        timestampUs: frame.timestamp,
        width: sample.displayWidth,
        height: sample.displayHeight,
        draw: (context, rectangle) => {
          if (released || this.closed) throw new MediaRangeError('CLIP_SOURCE_SUPERSEDED')
          context.save()
          try {
            context.translate(rectangle.x + rectangle.width / 2, rectangle.y + rectangle.height / 2)
            if (sample.flip) context.scale(-1, 1)
            context.rotate((sample.rotation * Math.PI) / 180)
            const odd = sample.rotation % 180 !== 0
            const width = odd ? rectangle.height : rectangle.width,
              height = odd ? rectangle.width : rectangle.height
            context.drawImage(resource.frame, -width / 2, -height / 2, width, height)
          } finally {
            context.restore()
          }
        },
        close: () => {
          if (released) return
          released = true
          resource.frame.close()
          release()
          this.leases.delete(resource)
        },
      }
      this.leases.add(resource)
      return resource
    } catch (error) {
      frame?.close()
      release()
      throw error
    }
  }
  measurements() {
    return { frameExports: this.frameExports, livePresentationFrames: this.leases.size }
  }
  async close() {
    if (this.closed) return
    this.closed = true
    for (const resource of this.leases) resource.close()
    this.current?.close()
    this.next?.close()
    this.current = this.next = undefined
    await this.iterator.return?.()
  }
}
