import type {
  ClipAnalysisOriginalMeasurement,
  ClipAnalysisPreparation,
  ClipQuote,
  ReadyClipBatch,
} from '@/entities/clip-project'
import type { BrowserMediaSourceAccess } from '@/shared/lib'

export type AnalysisCopySlot = ClipAnalysisPreparation['copies'][number]
export interface AnalysisSource {
  sourceId: string
  fingerprint: string
  access: BrowserMediaSourceAccess
  durationBudgetMs?: number
}
export interface AnalysisCopyArtifact {
  buffer: ArrayBuffer
  sha256: string
  inspection: {
    bytes: number
    videoFrames: number
    videoStartMs: number
    videoEndMs: number
    containerEndMs: number
    audioSamples: number
    audioStartMs: number
    audioEndMs: number
    width: number
    height: number
    rotation: number
    hasAudio: boolean
    targetVideoBitrate: number
    actualVideoBitrate: number
    actualAudioBitrate: number
  }
}
export interface AnalysisPreparationRequest {
  projectId: string
  revision: number
  batch: ReadyClipBatch
  quote: ClipQuote
}
export interface AnalysisPreparationProgress {
  stage: 'measuring' | 'starting' | 'encoding' | 'uploading' | 'verifying'
  done: number
  total: number
  sourceId?: string
  slot?: string
  fraction?: number
}
export interface AnalysisEncoder {
  measure(source: AnalysisSource): Promise<ClipAnalysisOriginalMeasurement>
  encode(source: AnalysisSource, slot: AnalysisCopySlot): Promise<AnalysisCopyArtifact>
  close(): void
}
export interface ClipBrowserPreparation {
  run(
    input: AnalysisPreparationRequest,
    startParent: (preparationId: string) => Promise<{ jobId: string }>,
  ): Promise<{ jobId: string }>
}
