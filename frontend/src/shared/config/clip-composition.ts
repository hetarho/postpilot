/** The portable grammar's finite bounds, mirrored by config.ClipCompositionLimits.
 * Shared corpus tests compare both owners; no deployment setting changes the grammar. */
export const CLIP_COMPOSITION_LIMITS = {
  sourceChars: 16000,
  nodes: 200,
  fields: 10,
  items: 20,
  cuts: 100,
  cues: 2400,
  labelChars: 40,
  promptChars: 200,
  answerChars: 500,
  copyChars: 500,
  guideChars: 4000,
  maxDurationMs: 90000,
  autoInsetMs: 120,
} as const

export const CLIP_COMPOSITION_PREVIEW = {
  durationMs: 30000,
  minDurationMs: 1000,
  stepMs: 100,
  expandedBytes: 262144,
  sampleItems: 2,
} as const

export const CLIP_DRAFT_PREVIEW = {
  // Mirrors rpcserver.maxRequestBytes; preview never widens the RPC boundary.
  maxRequestBytes: 256 * 1024,
  debounceMs: 250,
  maxAssets: 8,
  maxAssetBytes: 512 * 1024,
  maxResponseBytes: 4 * 1024 * 1024,
  cacheBytes: 32 * 1024 * 1024,
  frameToleranceMs: 1000 / 30,
  seekStepMs: 1,
} as const

export const CLIP_TIMELINE = {
  history: 100,
  coalesceMs: 700,
  autosaveMs: 750,
  framesPerSecond: 30,
  previewViewportFraction: 0.3,
  compactViewportHeight: 520,
  compactPreviewFraction: 0.2,
  fieldGapPx: 8,
  pixelsPerSecond: 80,
  minWidth: 320,
} as const
