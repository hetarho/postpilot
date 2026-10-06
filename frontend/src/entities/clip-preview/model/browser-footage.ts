import { CLIP_VIDEO_DECODING } from '../config/video-decoding'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import {
  openOriginalVideo,
  OriginalVideoCursor,
  VideoFrameBudget,
  MediaRangeError,
  type BrowserMediaSourceAccess,
  type DecodedVideoResource,
  type OriginalVideoInput,
  type OriginalVideoMetadata,
} from '@/shared/lib'
import {
  BrowserSnapshotEpoch,
  type BrowserCompositionSnapshot,
  type BrowserEvaluatedFrame,
} from './browser-composition'

type FootageLayer = BrowserEvaluatedFrame['footageLayers'][number]
export interface BrowserPreparedFootage {
  layer: FootageLayer
  resource: DecodedVideoResource
}
export type BrowserSourceAccess = (
  sourceId: string,
  fingerprint: string,
  signal: AbortSignal,
) => Promise<BrowserMediaSourceAccess>
export interface BrowserFootagePorts {
  open?: typeof openOriginalVideo
  fetch?: typeof fetch
  opened?: (milliseconds: number) => void
  read?: (bytes: number, milliseconds: number) => void
  decoded?: (milliseconds: number) => void
}

/** Clip-specific range identity/time policy. Generic codecs/resources stay in shared. */
export class BrowserFootageResources {
  private epoch = new BrowserSnapshotEpoch()
  private token
  private controller = new AbortController()
  private cursors = new Map<
    string,
    { cursor: OriginalVideoCursor; input: OriginalVideoInput; reservation: number }
  >()
  private owned = new Set<DecodedVideoResource>()
  private budget = new VideoFrameBudget(
    CLIP_VIDEO_DECODING.presentationFrames,
    CLIP_VIDEO_DECODING.presentationBytes,
  )
  private reserved = 0
  private reservations = new Map<string, number>()
  private peakReserved = 0
  private peakActive = 0
  private sourceReads = 0
  private sourceBytes = 0
  private sourceReadMs = 0
  private decodeMs = 0
  private openMs = 0
  private frameExports = 0
  private inputs = new Map<string, OriginalVideoMetadata>()
  private busy = false
  private disposed = false
  private abort = () => {
    void this.dispose()
  }
  constructor(
    private snapshot: BrowserCompositionSnapshot,
    private sourceAccess: BrowserSourceAccess,
    private signal: AbortSignal,
    private ports: BrowserFootagePorts = {},
  ) {
    this.token = this.epoch.begin(snapshot)
    signal.addEventListener('abort', this.abort, { once: true })
    if (signal.aborted) this.abort()
  }
  private check() {
    this.signal.throwIfAborted()
    this.controller.signal.throwIfAborted()
    this.epoch.assertCurrent(this.token)
  }
  private releaseReservation(id: string) {
    const bytes = this.reservations.get(id)
    if (bytes !== undefined) {
      this.reserved -= bytes
      this.reservations.delete(id)
    }
  }
  private async open(layer: FootageLayer) {
    this.check()
    const source = this.snapshot.sources.find(
      (s) => s.sourceId === layer.sourceId && s.fingerprint === layer.fingerprint,
    )
    if (!source || !source.allowedRatePermille.includes(layer.ratePermille))
      throw new MediaRangeError('CLIP_SOURCE_RATE_UNSUPPORTED')
    const estimate =
      source.width *
      source.height *
      CLIP_VIDEO_DECODING.bytesPerPixelReserve *
      CLIP_VIDEO_DECODING.decoderReserveFrames
    if (
      !Number.isSafeInteger(estimate) ||
      this.reserved + estimate > CLIP_VIDEO_DECODING.decoderReserveBytes
    )
      throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
    this.reserved += estimate
    this.reservations.set(layer.cutInstanceId, estimate)
    this.peakReserved = Math.max(this.peakReserved, this.reserved)
    const began = performance.now()
    let input: OriginalVideoInput | undefined
    const deadline = setTimeout(
      () => this.controller.abort(new MediaRangeError('CLIP_SOURCE_TIMEOUT')),
      CLIP_VIDEO_DECODING.timeoutMs,
    )
    try {
      const access = await this.sourceAccess(
        source.sourceId,
        source.fingerprint,
        this.controller.signal,
      )
      this.check()
      input = await (this.ports.open ?? openOriginalVideo)(
        access,
        CLIP_VIDEO_DECODING,
        this.controller.signal,
        {
          fetch: this.ports.fetch,
          read: (bytes, elapsed) => {
            this.sourceReads++
            this.sourceBytes += bytes
            this.sourceReadMs += elapsed
            this.ports.read?.(bytes, elapsed)
          },
        },
      )
      this.check()
      const metadata = input.metadata
      if (
        metadata.durationFromMetadata !== null &&
        layer.sourceEndUs > Math.round(metadata.durationFromMetadata * 1_000_000)
      )
        throw new MediaRangeError('CLIP_SOURCE_RANGE_UNSUPPORTED')
      if (
        metadata.displayWidth !== source.width ||
        metadata.displayHeight !== source.height ||
        !Number.isSafeInteger(metadata.codedWidth) ||
        metadata.codedWidth <= 0 ||
        !Number.isSafeInteger(metadata.codedHeight) ||
        metadata.codedHeight <= 0 ||
        !Number.isSafeInteger(metadata.timeResolution) ||
        metadata.timeResolution <= 0
      )
        throw new MediaRangeError('CLIP_SOURCE_METADATA_MISMATCH')
      const reservation =
        metadata.codedWidth *
        metadata.codedHeight *
        CLIP_VIDEO_DECODING.bytesPerPixelReserve *
        CLIP_VIDEO_DECODING.decoderReserveFrames
      if (
        !Number.isSafeInteger(reservation) ||
        this.reserved - estimate + reservation > CLIP_VIDEO_DECODING.decoderReserveBytes
      )
        throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
      this.reserved += reservation - estimate
      this.reservations.set(layer.cutInstanceId, reservation)
      this.peakReserved = Math.max(this.peakReserved, this.reserved)
      this.inputs.set(source.sourceId, { ...metadata })
      const cursor = new OriginalVideoCursor(
        input,
        {
          startUs: layer.sourceStartUs,
          endUs: layer.sourceEndUs,
          ratePermille: layer.ratePermille,
          fps: CLIP_BROWSER_RENDER.frameRate,
        },
        this.budget,
        this.controller.signal,
        {
          decode: (elapsed) => {
            this.decodeMs += elapsed
            this.ports.decoded?.(elapsed)
          },
        },
      )
      this.cursors.set(layer.cutInstanceId, { input, cursor, reservation })
      this.peakActive = Math.max(this.peakActive, this.cursors.size)
      return cursor
    } catch (error) {
      input?.dispose()
      this.releaseReservation(layer.cutInstanceId)
      throw error
    } finally {
      clearTimeout(deadline)
      const elapsed = performance.now() - began
      this.openMs += elapsed
      this.ports.opened?.(elapsed)
    }
  }
  async prepare(frame: BrowserEvaluatedFrame): Promise<BrowserPreparedFootage[]> {
    this.check()
    if (this.busy) throw new MediaRangeError('CLIP_SOURCE_QUEUE_LIMIT')
    if (frame.footageLayers.length > CLIP_VIDEO_DECODING.activeCuts)
      throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
    this.busy = true
    const prepared: BrowserPreparedFootage[] = []
    try {
      const active = new Set(frame.footageLayers.map((layer) => layer.cutInstanceId))
      for (const [id, entry] of this.cursors)
        if (!active.has(id)) {
          await entry.cursor.close()
          entry.input.dispose()
          this.releaseReservation(id)
          this.cursors.delete(id)
        }
      for (const layer of frame.footageLayers) {
        const cursor = this.cursors.get(layer.cutInstanceId)?.cursor ?? (await this.open(layer))
        const deadline = setTimeout(
          () => this.controller.abort(new MediaRangeError('CLIP_SOURCE_TIMEOUT')),
          CLIP_VIDEO_DECODING.timeoutMs,
        )
        let resource: DecodedVideoResource
        try {
          resource = await cursor.frame(layer.cutLocalFrame)
        } finally {
          clearTimeout(deadline)
        }
        this.check()
        this.frameExports++
        this.owned.add(resource)
        const originalClose = resource.close
        resource.close = () => {
          originalClose()
          this.owned.delete(resource)
        }
        prepared.push({ layer, resource })
      }
      this.check()
      return prepared
    } catch (error) {
      for (const entry of prepared) entry.resource.close()
      await this.dispose()
      throw error
    } finally {
      this.busy = false
    }
  }
  /** A seek ends old decoders and callbacks; caller creates a fresh bounded range owner. */
  supersede() {
    this.epoch.cancel()
    return this.dispose()
  }
  measurements() {
    return {
      sourceReads: this.sourceReads,
      sourceBytes: this.sourceBytes,
      sourceReadMs: this.sourceReadMs,
      decodeMs: this.decodeMs,
      openMs: this.openMs,
      peakActiveCuts: this.peakActive,
      activeCuts: this.cursors.size,
      peakDecoderReservedBytes: this.peakReserved,
      decoderReservedBytes: this.reserved,
      presentation: this.budget.snapshot(),
      frameExports: this.frameExports,
      bitmapCopies: 0,
      physicalPeakBytes: null,
      originals: [...this.inputs.values()],
    }
  }
  async dispose() {
    if (this.disposed) return
    this.disposed = true
    this.signal.removeEventListener('abort', this.abort)
    this.epoch.cancel()
    this.controller.abort()
    for (const resource of this.owned) resource.close()
    // Close Inputs first to interrupt pending decoder/reader operations before
    // awaiting iterator return. Decoder siblings stop together on any failure.
    for (const entry of this.cursors.values()) entry.input.dispose()
    await Promise.allSettled([...this.cursors.values()].map((entry) => entry.cursor.close()))
    this.cursors.clear()
    this.reservations.clear()
    this.reserved = 0
  }
}
