/** The draft preview and the browser render: what the plan looks like before it is a file. */
export { clipPreviewRequest, useClipPreviewRequest } from './api/preview'
export type { ClipPreviewRequest } from './api/preview'
export { useClipBrowserRenderCapability } from './api/useClipBrowserRenderCapability'
export { createClipVideoWorker } from './lib/create-video-worker'
export { browserAudioPlan } from './model/browser-audio-plan'
export { BrowserFootageResources } from './model/browser-footage'
export type {
  BrowserPreparedFootage,
  BrowserSourceAccess,
  BrowserFootagePorts,
} from './model/browser-footage'
export { CLIP_VIDEO_DECODING } from './config/video-decoding'
export { CLIP_AUDIO_PROCESSING } from './config/audio-processing'
export {
  BrowserCompositionError,
  BrowserSnapshotEpoch,
  evaluateBrowserFrame,
  freezeBrowserComposition,
  readBrowserCompositionSnapshot,
} from './model/browser-composition'
export type {
  BrowserCompositionSnapshot,
  BrowserCompositionInput,
  BrowserCompositionVersions,
  BrowserCompositionDesign,
  BrowserCompositionComponent,
  BrowserSourceIdentity,
  BrowserEvaluatedFrame,
  BrowserMediaResolver,
  BrowserSnapshotToken,
  BrowserFrozen,
} from './model/browser-composition'
export type { SpeechAudioLoader } from './model/speech-playback'
export { canonicalSpeechBuffer, speechDecodeKey, SpeechDecodeCache } from './model/speech-playback'
export { speechRenderFingerprint } from './model/speech-fingerprint'
export { clipBrowserEncoderConfig, clipRenderNeedsAudio } from './model/browser-render-capability'
export type { ClipBrowserRenderCapability } from './model/browser-render-capability'
export type {
  BrowserVideoInput,
  BrowserVideoProgress,
  BrowserVideoRender,
  BrowserVideoTrack,
  EncodedClipChunk,
} from './model/browser-video'
export { previewElementIDs, previewTimeline, previewMotion } from './model/draft-preview'
export type { ClipPreviewOverlay } from './model/draft-preview'
export { CaptionSheets } from './model/caption-sheets'
export type { CaptionCell, CaptionFrameLoader, CaptionFramePage } from './model/caption-sheets'
export { PreviewAssetCache, PreviewPreparation } from './model/preview-assets'
export type { PreparedAsset } from './model/preview-assets'
export type { VideoWorkerInput, VideoWorkerOutput } from './model/video-worker-protocol'
export { ClipDraftPreview } from './ui/ClipDraftPreview'
export type { ClipDisplayedFrame } from './ui/ClipDraftPreview'
export { useClipRenderCalls } from './api/render'
export type { ClipRenderCalls, ClipRenderMachine, ClipRenderVerdict } from './api/render'
export { BrowserLocalComponents } from './model/local-components'
export type { BrowserLocalComponent } from './model/local-components'
export { BrowserInkCache } from './model/ink-cache'
export type { BrowserInkLease } from './model/ink-cache'
export { ResvgBrowserInk } from './model/ink-raster'
export type { BrowserInkRasterizer, InkDocument } from './model/ink-raster'
export { inkLayoutCaption } from './model/ink-layout'
export type {
  InkCaptionInput,
  InkCaptionLayout,
  InkCaptionLine,
  InkCaptionWord,
} from './model/ink-layout'
export {
  inkStaticCaption,
  inkStaticRegion,
  inkStaticBadge,
  inkStaticInfo,
} from './model/ink-static'
export type { InkPaint, InkRegionPart } from './model/ink-static'
export { ClipInkError } from './model/ink-typography'
export { inkCaptionScene, inkPopProgress } from './model/ink-caption-scene'
export type { InkCaptionScene, InkCaptionPose, InkMatrix } from './model/ink-caption-scene'
export { BrowserCaptionScenePixi } from './model/ink-caption-pixi'
export type { BrowserCaptionPreparedScene, BrowserCaptionSceneNode } from './model/ink-caption-draw'
export { BrowserCaptionSceneCanvas } from './model/ink-caption-draw'
