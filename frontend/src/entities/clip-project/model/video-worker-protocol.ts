import type { BrowserVideoInput, BrowserVideoProgress, BrowserVideoTrack } from './browser-video'

export type VideoWorkerInput =
  | { type: 'start'; input: BrowserVideoInput }
  | { type: 'source'; requestId: number; bitmap: ImageBitmap }
export type VideoWorkerOutput =
  | { type: 'source'; requestId: number; fingerprint: string; timeMs: number }
  | { type: 'progress'; progress: BrowserVideoProgress }
  | { type: 'done'; track: BrowserVideoTrack }
  | { type: 'error'; error: string }
