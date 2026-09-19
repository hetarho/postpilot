/** The draft preview and the browser render: what the plan looks like before it is a file. */
export { clipPreviewRequest, useClipPreviewRequest } from './api/preview'
export type { ClipPreviewRequest } from './api/preview'
export { useClipBrowserRenderCapability } from './api/useClipBrowserRenderCapability'
export { createClipVideoWorker } from './lib/create-video-worker'
export { browserAudioPlan } from './model/browser-audio-plan'
export { clipBrowserEncoderConfig, clipRenderNeedsAudio } from './model/browser-render-capability'
export type { ClipBrowserRenderCapability } from './model/browser-render-capability'
export type {
  BrowserVideoInput,
  BrowserVideoProgress,
  BrowserVideoRender,
  BrowserVideoTrack,
  EncodedClipChunk,
} from './model/browser-video'
export { previewElementIDs, previewTimeline } from './model/draft-preview'
export type { ClipPreviewOverlay } from './model/draft-preview'
export { PreviewAssetCache, PreviewPreparation } from './model/preview-assets'
export type { PreparedAsset } from './model/preview-assets'
export type { VideoWorkerInput, VideoWorkerOutput } from './model/video-worker-protocol'
export { ClipDraftPreview } from './ui/ClipDraftPreview'
export type { ClipDisplayedFrame } from './ui/ClipDraftPreview'
export { useClipRenderCalls } from './api/render'
export type { ClipRenderCalls, ClipRenderMachine, ClipRenderVerdict } from './api/render'
