/** The portable grammar's finite bounds, mirrored by config.ClipCompositionLimits.
 * Shared corpus tests compare both owners; no deployment setting changes the grammar. */
export const CLIP_COMPOSITION_LIMITS = {
  sourceChars: 16000,
  nodes: 200,
  fields: 10,
  items: 20,
  cuts: 100,
  cues: 2400,
  stages: 8,
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
  /** How long the preview's one sample narration line stays on screen. */
  narrationMs: 3000,
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

/** How long the revision request waits after the last keystroke before it is
 *  priced. A quote binds the exact words it was taken against (CLIP-131), so the
 *  text is part of its key; without this every character would ask the server. */
export const CLIP_REVISION = {
  quoteDebounceMs: 700,
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
  minimumEditingRoomPx: 96,
  pixelsPerSecond: 80,
  minWidth: 320,
} as const

/** How ② places a caption (CDS-82, CLIP-143). The nudge is in CANVAS pixels, so
 *  a keystroke moves the caption by the same amount at every ratio and at every
 *  size the stage happens to be drawn at. */
export const CLIP_CAPTION_PLACEMENT = {
  nudgePx: 8,
  coarseNudgePx: 40,
} as const
