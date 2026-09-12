import type { ClipEditPlan, RetainedClipSource } from './edit-plan'

export interface ClipObservedSegment {
  startMs: number
  endMs: number
  event: string
  subjects: string[]
  speech: string
  quality: string
}
export interface ClipSourceObservation {
  source: RetainedClipSource
  segments: ClipObservedSegment[]
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
    return startMs < endMs ? [{ cutId: cut.id, number: index + 1, startMs, endMs }] : []
  })
}

/** Reuse recorded evidence, without making another model call to summarize it. */
export function observationSummary(observation: ClipSourceObservation): string {
  for (const segment of observation.segments) {
    const summary = segment.event.trim() || segment.speech.trim() || segment.subjects.join(', ')
    if (summary) return summary
  }
  return ''
}
