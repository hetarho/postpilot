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
