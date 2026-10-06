import type { RetainedClipSource } from '@/entities/clip-plan/@x/clip-preview'
import type { ClipLayoutObservations } from '@/entities/clip-observation/@x/clip-preview'
import { BrowserCompositionError, type BrowserCompositionInput } from './browser-composition'

export interface BrowserProjectCompositionInput extends Omit<
  BrowserCompositionInput,
  'sources' | 'layoutObservations'
> {
  sources: readonly RetainedClipSource[]
  layoutObservations?: ClipLayoutObservations
}
/** Allowlisted read projection. Portable layout authority never comes from editable text. */
export function projectBrowserComposition(
  input: BrowserProjectCompositionInput,
): BrowserCompositionInput {
  if (input.layoutObservations?.status === 'unavailable')
    throw new BrowserCompositionError('CLIP_LAYOUT_OBSERVATIONS_UNAVAILABLE')
  return {
    ownerId: input.ownerId,
    projectId: input.projectId,
    projectRevision: input.projectRevision,
    planRevision: input.planRevision,
    plan: input.plan,
    ratio: input.ratio,
    design: input.design,
    versions: input.versions,
    authoritativeFingerprint: input.authoritativeFingerprint,
    sources: input.sources.map((source) => ({
      sourceId: source.id,
      fingerprint: source.fingerprint,
      durationMs: source.durationMs,
      width: source.width,
      height: source.height,
      ...(source.hasAudio !== undefined ? { hasAudio: source.hasAudio } : {}),
      originalMeasurementProvenance: source.originalMeasurementProvenance ?? '',
      allowedRatePermille: [...source.allowedRatePermille],
    })),
    layoutObservations: (input.layoutObservations?.sources ?? []).map(({ source, segments }) => ({
      sourceId: source.id,
      fingerprint: source.fingerprint,
      durationMs: source.durationMs,
      width: source.width,
      height: source.height,
      originalMeasurementProvenance: source.originalMeasurementProvenance ?? '',
      segments: segments.map((segment) => ({
        startMs: segment.startMs,
        endMs: segment.endMs,
        scene: segment.scene ?? '',
        readableText: segment.readableText ?? false,
        subject: segment.subject ? { ...segment.subject } : { x: 0, y: 0, width: 0, height: 0 },
        captionSafe: (segment.captionSafe ?? []).map((box) => ({ ...box })),
      })),
    })),
  }
}
