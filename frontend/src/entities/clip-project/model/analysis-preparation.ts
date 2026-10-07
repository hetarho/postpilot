export const CLIP_BROWSER_ANALYSIS_PROFILE = 'clip-browser-analysis-v1'

export interface ClipAnalysisOriginalMeasurement {
  provenance: 'browser_client'
  sourceId: string
  fingerprint: string
  durationMs: number
  width: number
  height: number
  frameRateNumerator: number
  frameRateDenominator: number
  cadenceVerified: boolean
  decodedFrames: number
  hasAudio: boolean
  audioRate: number
  audioChannels: number
}
export interface ClipAnalysisPreparationInput {
  projectId: string
  batchId: string
  expectedRevision: number
  quoteId: string
  profileVersion: string
  originals: ClipAnalysisOriginalMeasurement[]
}
export type ClipAnalysisPreparationState =
  'preparing' | 'verifying' | 'accepted' | 'consumed' | 'failed' | 'cancelled' | 'expired'
export interface ClipAnalysisPreparation {
  id: string
  projectId: string
  batchId: string
  revision: number
  state: ClipAnalysisPreparationState
  expiresAt: string
  originalMeasurementProvenance: 'browser_client'
  profile: {
    version: string
    intervalMs: number
    longEdge: number
    framesPerSecond: number
    maxCopyBytes: number
    videoCodec: string
    pixelFormat: string
    audioCodec: string
    audioRate: number
    audioChannels: number
    audioBitrate: number
    qualified: boolean
  }
  copies: {
    slot: string
    sourceId: string
    fingerprint: string
    ordinal: number
    offsetMs: number
    durationMs: number
    width: number
    height: number
    hasAudio: boolean
    state: 'expected' | 'reserved' | 'verified' | 'cleanup' | 'deleted'
    bytes: number
    sha256: string
  }[]
  progress: number
  failure: string
  jobId: string
}
