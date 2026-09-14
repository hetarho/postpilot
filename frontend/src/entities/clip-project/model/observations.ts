import { cutRate, type ClipEditPlan, type RetainedClipSource } from './edit-plan'

/** certain | uncertain | unknown, and usable | unusable, exactly as recorded.
 * 'unspecified' is what a record written before the v2 contract carries: the
 * viewer says so rather than showing an invented certainty (CLIP-51). */
export type ClipObservationCertainty = 'certain' | 'uncertain' | 'unknown' | 'unspecified'
export type ClipObservationUsability = 'usable' | 'unusable' | 'unspecified'

export interface ClipObservedSegment {
  focal?: { x: number; y: number }
  startMs: number
  endMs: number
  event: string
  action: string
  motion: string
  subjects: string[]
  speech: string
  quality: string
  certainty: ClipObservationCertainty
  usability: ClipObservationUsability
}
export interface ClipSourceObservation {
  source: RetainedClipSource
  segments: ClipObservedSegment[]
}
export interface ClipAddCutSelection {
  source: RetainedClipSource
  segment: ClipObservedSegment
  startMs: number
  endMs: number
}
export interface ClipObservations {
  status: 'available' | 'empty' | 'unavailable'
  sources: ClipSourceObservation[]
}

/** Intersections with the SAVED plan, never an unsaved draft or an inferred reason.
 * A repeated filename cannot attach another source's cuts, and touching is not use. */
export function observationCutUsage(
  segment: ClipObservedSegment,
  source: RetainedClipSource,
  plan: ClipEditPlan | undefined,
) {
  return (plan?.cuts ?? []).flatMap((cut, index) => {
    if (cut.sourceId !== source.id || cut.fingerprint !== source.fingerprint) return []
    const startMs = Math.max(segment.startMs, cut.startMs)
    const endMs = Math.min(segment.endMs, cut.endMs)
    return startMs < endMs
      ? [{ cutId: cut.id, number: index + 1, startMs, endMs, playbackRatePermille: cutRate(cut) }]
      : []
  })
}

/** Reuse recorded evidence, without making another model call to summarize it. */
export function observationSummary(observation: ClipSourceObservation): string {
  for (const segment of observation.segments) {
    const summary =
      segment.event.trim() ||
      segment.action.trim() ||
      segment.speech.trim() ||
      segment.subjects.join(', ')
    if (summary) return summary
  }
  return ''
}
