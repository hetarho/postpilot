import type {
  ClipAnalysisOriginalMeasurement,
  ClipAnalysisPreparation,
} from '@/entities/clip-project'
import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'
import type { AnalysisCopyArtifact, AnalysisCopySlot } from './types'

export function analysisGeometry(width: number, height: number) {
  const scale = Math.min(1, limits.longEdge / Math.max(width, height))
  return {
    width: Math.max(2, Math.floor((width * scale) / 2) * 2),
    height: Math.max(2, Math.floor((height * scale) / 2) * 2),
  }
}
export function validateOriginalMeasurements(
  originals: readonly ClipAnalysisOriginalMeasurement[],
) {
  let total = 0
  if (!originals.length || originals.length > limits.sources)
    throw new Error('CLIP_INPUT_TOO_LARGE')
  for (const original of originals) {
    if (
      original.provenance !== 'browser_client' ||
      ![
        original.width,
        original.height,
        original.frameRateNumerator,
        original.frameRateDenominator,
        original.audioRate,
        original.audioChannels,
      ].every(Number.isSafeInteger) ||
      !Number.isSafeInteger(original.durationMs) ||
      original.durationMs < 1 ||
      !Number.isSafeInteger(original.decodedFrames) ||
      original.decodedFrames < 1 ||
      Math.min(original.width, original.height) < 2 ||
      Math.max(original.width, original.height) > limits.maxDimension ||
      original.frameRateNumerator < 1 ||
      original.frameRateDenominator < 1 ||
      original.frameRateNumerator > 1000000 ||
      original.frameRateDenominator > 1000000 ||
      (original.hasAudio &&
        (original.audioRate < 8000 ||
          original.audioRate > 192000 ||
          original.audioChannels < 1 ||
          original.audioChannels > 16)) ||
      (!original.hasAudio && (original.audioRate !== 0 || original.audioChannels !== 0))
    )
      throw new Error('CLIP_SOURCE_MEASUREMENT_INVALID')
    total += original.durationMs
  }
  if (total > limits.durationMs) throw new Error('CLIP_INPUT_TOO_LARGE')
}
/** The server decides compatible retained coverage. Missing intervals may have
 * gaps, but no duplicated, overlapping or geometry-changing slot is accepted. */
export function validateMissingSlots(
  preparation: ClipAnalysisPreparation,
  originals: readonly ClipAnalysisOriginalMeasurement[],
) {
  const seen = new Set<string>()
  for (const copy of preparation.copies) {
    const original = originals.find((source) => source.sourceId === copy.sourceId)
    if (
      !original ||
      copy.fingerprint !== original.fingerprint ||
      seen.has(`${copy.sourceId}/${copy.ordinal}`)
    )
      throw new Error('CLIP_ANALYSIS_COPY_OWNERSHIP')
    seen.add(`${copy.sourceId}/${copy.ordinal}`)
    const geometry = analysisGeometry(original.width, original.height)
    if (
      copy.offsetMs !== copy.ordinal * limits.intervalMs ||
      copy.offsetMs >= original.durationMs ||
      copy.durationMs !== Math.min(limits.intervalMs, original.durationMs - copy.offsetMs) ||
      copy.width !== geometry.width ||
      copy.height !== geometry.height ||
      copy.hasAudio !== original.hasAudio ||
      copy.state !== 'expected'
    )
      throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
  }
  if (preparation.copies.length > limits.copies) throw new Error('CLIP_INPUT_TOO_LARGE')
}
export function validateAnalysisArtifact(slot: AnalysisCopySlot, artifact: AnalysisCopyArtifact) {
  const p = artifact.inspection
  const videoTolerance = 1000 / limits.fps + 0.01
  if (
    !Object.values(p)
      .filter((value): value is number => typeof value === 'number')
      .every(Number.isFinite) ||
    !Number.isSafeInteger(p.videoFrames) ||
    !Number.isSafeInteger(p.audioSamples) ||
    artifact.buffer.byteLength < 1 ||
    artifact.buffer.byteLength > limits.maxCopyBytes ||
    p.bytes !== artifact.buffer.byteLength ||
    !/^[a-f0-9]{64}$/.test(artifact.sha256) ||
    p.width !== slot.width ||
    p.height !== slot.height ||
    p.rotation !== 0 ||
    p.hasAudio !== slot.hasAudio ||
    p.videoFrames < 1 ||
    p.videoFrames > limits.fps * 60 ||
    Math.abs((p.videoFrames * 1000) / limits.fps - slot.durationMs) > videoTolerance ||
    Math.abs(p.videoStartMs) > 0.01 ||
    p.containerEndMs > limits.intervalMs + 0.01 ||
    Math.abs(p.containerEndMs - slot.durationMs) > videoTolerance ||
    p.videoEndMs > limits.intervalMs + 0.01 ||
    Math.abs(p.videoEndMs - slot.durationMs) > videoTolerance ||
    (slot.hasAudio &&
      (Math.abs(p.audioStartMs) > 22 ||
        Math.abs(p.audioEndMs - slot.durationMs) > 22 ||
        Math.abs(p.audioSamples - (slot.durationMs * limits.audioRate) / 1000) > 1024)) ||
    (!slot.hasAudio && p.audioSamples !== 0)
  )
    throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
}
