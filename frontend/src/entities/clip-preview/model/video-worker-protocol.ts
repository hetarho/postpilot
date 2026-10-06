import type {
  BrowserVideoInput,
  BrowserVideoProgress,
  BrowserVideoTrack,
  BrowserVideoMeasurements,
} from './browser-video'
import type { BrowserMediaSourceAccess } from '@/shared/lib'

export type VideoWorkerInput =
  | { type: 'start'; input: BrowserVideoInput }
  | { type: 'cancel' }
  | { type: 'sourceAccess'; requestId: number; access?: BrowserMediaSourceAccess; error?: string }
  | { type: 'source'; requestId: number; bitmap: ImageBitmap }
  // A sequence caption's own frame for this output frame, drawn by the server and
  // placed at the origin its overlay uses (CLIP-159). `bitmap` is absent where the
  // caption has no frame there.
  | {
      type: 'frames'
      requestId: number
      bitmap?: ImageBitmap
      x: number
      y: number
      width: number
      height: number
    }
export type VideoWorkerOutput =
  | { type: 'sourceAccess'; requestId: number; sourceId: string; fingerprint: string }
  | { type: 'cancelled' }
  | { type: 'source'; requestId: number; fingerprint: string; timeMs: number }
  | { type: 'frames'; requestId: number; instanceId: string; frame: number }
  | { type: 'progress'; progress: BrowserVideoProgress }
  | { type: 'done'; track: BrowserVideoTrack }
  | { type: 'error'; error: string; measurements?: BrowserVideoMeasurements }
