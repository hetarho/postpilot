import { measureOriginalMedia, transcodeMediaInterval } from '@/shared/lib'
import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'
import { validateAnalysisArtifact } from '../model/coverage'
import type { AnalysisCopySlot, AnalysisSource } from '../model/types'

export async function measureAnalysisOriginal(source: AnalysisSource, signal: AbortSignal) {
  return {
    sourceId: source.sourceId,
    fingerprint: source.fingerprint,
    ...(await measureOriginalMedia(source.access, limits, signal)),
  }
}
export async function encodeAnalysisCopy(
  source: AnalysisSource,
  slot: AnalysisCopySlot,
  signal: AbortSignal,
  progress: (fraction: number) => void = () => {},
) {
  const artifact = await transcodeMediaInterval(source.access, slot, limits, signal, progress)
  validateAnalysisArtifact(slot, artifact)
  return artifact
}
