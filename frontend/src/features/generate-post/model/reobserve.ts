import type { PostImage } from '@/entities/image'
import { observationByFile } from '@/entities/observation'
import type { Observation } from '@/shared/api'

/** One attached photo as the re-observation picker reasons about it. */
export interface ReobserveRow {
  filename: string
  /** The photo itself, so the picker renders a thumbnail without re-pairing by filename. */
  image: PostImage
  /** The observation currently stored for this photo, or undefined when there is none. */
  stored?: Observation
  /** Nothing to reuse: no stored entry, or one a model produced without seeing anything.
   *  Such a photo is observed whether or not the user asks — the run must not write from a
   *  photo nothing has looked at — so its checkbox is checked and cannot be cleared. */
  forced: boolean
}

/** True when the entry carries no eyesight. `file` and `model` are deliberately not part of
 *  this: one names the photo and the other names who looked, and neither is something the
 *  writing stage can write from. Mirrors `observationEmpty` in the generation context, which
 *  is what actually enforces the rule. */
function empty(observation: Observation): boolean {
  return (
    !observation.scene &&
    !observation.mood &&
    !observation.visibleText &&
    observation.objects.length === 0 &&
    !observation.peoplePresent
  )
}

/** One row per attached photo, in the post's own order. */
export function reobserveRows(
  images: readonly PostImage[],
  observations: readonly Observation[],
): ReobserveRow[] {
  const byFile = observationByFile(observations)
  return images.map((image) => {
    const stored = byFile.get(image.filename)
    return {
      filename: image.filename,
      image,
      stored: stored && !empty(stored) ? stored : undefined,
      forced: !stored || empty(stored),
    }
  })
}

/** What the picker opens with: the forced photos and nothing else. Every other checkbox
 *  starts CLEAR, so confirming without touching anything reuses every stored observation —
 *  the common case is a writing stage that failed on photos that did not change. */
export function defaultSelection(rows: readonly ReobserveRow[]): string[] {
  return rows.filter((row) => row.forced).map((row) => row.filename)
}

/** Whether starting a run on this post has a reuse decision to make. False means there is
 *  nothing to reuse, so the run would observe everything anyway and the picker would only be
 *  a confirmation of the one available answer: start directly, as before the picker existed. */
export function needsPicker(
  images: readonly PostImage[],
  observations: readonly Observation[],
): boolean {
  if (images.length === 0) return false
  return reobserveRows(images, observations).some((row) => !row.forced)
}

/** The model refs behind the observations the picker is offering to reuse, deduplicated and
 *  in row order. An entry whose provenance predates the field reports as unknown, which the
 *  caller renders in words rather than as an empty string. */
export function storedObservationModels(rows: readonly ReobserveRow[]): string[] {
  const models: string[] = []
  for (const row of rows) {
    if (!row.stored) continue
    const model = row.stored.model
    if (!models.includes(model)) models.push(model)
  }
  return models
}
