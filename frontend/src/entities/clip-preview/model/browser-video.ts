import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import type { ClipRatio } from '@/entities/clip-project/@x/clip-preview'
import type { PreparedAsset } from './preview-assets'
import type { MediaPhaseSnapshot } from '@/shared/lib'
import type { CLIP_VIDEO_MEASUREMENT_PHASES } from '../config/render-measurements'

export type BrowserVideoMeasurements = MediaPhaseSnapshot<
  (typeof CLIP_VIDEO_MEASUREMENT_PHASES)[number]
>

export interface BrowserVideoInput {
  plan: ClipEditPlan
  ratio: ClipRatio
  assets: PreparedAsset[]
  /** Diagnostic-only observations; never part of a saved plan or server verdict. */
  collectMeasurements?: boolean
}
export interface BrowserVideoProgress {
  completedFrames: number
  totalFrames: number
}
export interface EncodedClipChunk {
  type: EncodedVideoChunkType
  timestamp: number
  duration: number
  data: Uint8Array<ArrayBuffer>
}
export interface BrowserVideoTrack {
  config: VideoEncoderConfig
  decoderConfig: VideoDecoderConfig
  chunks: EncodedClipChunk[]
  frameCount: number
  durationUs: number
  measurements?: BrowserVideoMeasurements
}
export interface BrowserVideoRender {
  progress: AsyncIterable<BrowserVideoProgress>
  result: Promise<BrowserVideoTrack>
  cancel: () => void
}
