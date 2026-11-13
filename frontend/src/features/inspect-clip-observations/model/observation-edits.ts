import type { ClipObservationEdit } from '@/entities/clip-project'
import type { ClipObservedSegment } from '@/entities/clip-observation'

/** The descriptive text the owner may correct in place; the time range, caption-safe regions,
 *  quality and the classifications the rate rules read stay as observed (CLIP-235). */
export const OBSERVATION_FIELDS = ['event', 'action', 'subjects', 'speech'] as const
export type ObservationField = (typeof OBSERVATION_FIELDS)[number]

/** The correction one field's typed value makes, or undefined when it cannot be saved: speech the
 *  scene was observed with cannot be cleared, because whether speech is audible is a
 *  classification the rate rules read (CLIP-235, CLIP-129). Subjects are typed one per line. */
export function observationEditOf(
  observationId: string,
  field: ObservationField,
  segment: ClipObservedSegment,
  value: string,
): ClipObservationEdit | undefined {
  if (field === 'subjects')
    return {
      observationId,
      subjects: value
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean),
    }
  if (field === 'speech' && segment.speech.trim() && !value.trim()) return undefined
  return { observationId, [field]: value }
}

/** A scene's id in the project's analysis: its source and its position there. */
export function observationId(sourceId: string, index: number): string {
  return `${sourceId}/${index}`
}
