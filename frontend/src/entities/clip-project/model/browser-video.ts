import type { ClipEditPlan } from './edit-plan'
import type { ClipRatio } from './types'
import type { PreparedAsset } from './preview-assets'

export interface BrowserVideoInput {
  plan: ClipEditPlan
  ratio: ClipRatio
  assets: PreparedAsset[]
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
}
export interface BrowserVideoRender {
  progress: AsyncIterable<BrowserVideoProgress>
  result: Promise<BrowserVideoTrack>
  cancel: () => void
}
