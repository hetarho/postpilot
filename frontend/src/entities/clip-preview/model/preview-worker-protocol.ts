import type { BrowserCompositionSnapshot } from './browser-composition'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
export interface BrowserPreviewFrameRequest {
  frame: number
  flow: boolean
  captionPosition?: { instanceId: string; x: number; y: number }
}
export interface BrowserPreviewFrameResult {
  bitmap: ImageBitmap
  frame: number
  flow: boolean
  fingerprint: string
  displayed: { cutId: string; sourceMs: number; outputMs: number }[]
  resources: { activeCuts: number; liveFrames: number; decoderReservedBytes: number }
  backgroundMeasured: boolean
}
export type BrowserPreviewWorkerInput =
  | { type: 'initialize'; id: number; snapshot: BrowserCompositionSnapshot }
  | ({ type: 'frame'; id: number; epoch: number } & BrowserPreviewFrameRequest)
  | { type: 'cancel'; epoch: number }
  | { type: 'source'; id: number; access?: BrowserMediaSourceAccess; error?: string }
  | { type: 'dispose' }
export type BrowserPreviewWorkerOutput =
  | { type: 'initialized'; id: number }
  | { type: 'rendered'; id: number; epoch: number; result: BrowserPreviewFrameResult }
  | { type: 'failed'; id: number; error: string }
  | {
      type: 'source'
      id: number
      sourceId: string
      fingerprint: string
      snapshotFingerprint: string
    }
