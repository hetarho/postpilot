import type { PostImage } from '@/entities/image'
import type { PostVideo } from '@/entities/video'
import { observationByFile } from '@/entities/post'
import type { Observation } from '@/shared/api'

/** Which kind a row is, carrying the attachment itself so the picker renders it without
 *  re-pairing by filename. Videos are listed after photos and behave identically otherwise:
 *  the picker's decision is about eyesight, and a clip has the same kind of it (VIDEO-18). */
export type ReobserveAttachment =
  { kind: 'photo'; image: PostImage } | { kind: 'video'; video: PostVideo }

/** One attached photo or clip as the re-observation picker reasons about it. */
export interface ReobserveRow {
  filename: string
  attachment: ReobserveAttachment
  /** The observation currently stored for this photo, or undefined when there is none. */
  stored?: Observation
  /** Nothing to reuse: no stored entry, or one a model produced without seeing anything.
   *  Such a photo is observed whether or not the user asks — the run must not write from a
   *  photo nothing has looked at — so its checkbox is checked and cannot be cleared. */
  forced: boolean
  /** The owner corrected this entry by hand (POST-126): it never counts as missing, and ticking
   *  it means those edits will be lost, which the picker confirms once (GEN-8). */
  ownerEdited: boolean
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
    !observation.peoplePresent &&
    // A clip can be carried by what only a clip has: a model that saw motion and heard
    // speech but named no objects still described it (VIDEO-9).
    observation.events.length === 0 &&
    !observation.speech
  )
}

/** One row per attachment, in the post's own order: photos first, then clips (VIDEO-18). */
export function reobserveRows(
  images: readonly PostImage[],
  observations: readonly Observation[],
  videos: readonly PostVideo[] = [],
): ReobserveRow[] {
  const byFile = observationByFile(observations)
  const row = (filename: string, attachment: ReobserveAttachment): ReobserveRow => {
    const stored = byFile.get(filename)
    return {
      filename,
      attachment,
      stored: stored && (stored.ownerEdited || !empty(stored)) ? stored : undefined,
      // An owner-edited entry is the owner's material even when every field was cleared (GEN-90).
      forced: !stored || (!stored.ownerEdited && empty(stored)),
      ownerEdited: Boolean(stored?.ownerEdited),
    }
  }
  return [
    ...images.map((image) => row(image.filename, { kind: 'photo', image })),
    ...videos.map((video) => row(video.filename, { kind: 'video', video })),
  ]
}

/** What the picker opens with: the forced photos and nothing else. Every other checkbox
 *  starts CLEAR, so confirming without touching anything reuses every stored observation —
 *  the common case is a writing stage that failed on photos that did not change. */
export function defaultSelection(rows: readonly ReobserveRow[]): string[] {
  return rows.filter((row) => row.forced).map((row) => row.filename)
}

/** Whether an attached photo or clip has no observation to reuse, so ①'s big action reads
 *  관찰하기 rather than 다음 (POST-123). An owner-edited entry never counts as missing. */
export function lacksObservation(
  images: readonly PostImage[],
  observations: readonly Observation[],
  videos: readonly PostVideo[] = [],
): boolean {
  return reobserveRows(images, observations, videos).some((row) => row.forced)
}

/** The ticked rows whose observation the owner edited by hand, whose edits a re-observation
 *  replaces — the ones the picker's confirmation names (GEN-8, POST-126). */
export function editedInSelection(
  rows: readonly ReobserveRow[],
  selection: readonly string[],
): string[] {
  return rows
    .filter((row) => row.ownerEdited && selection.includes(row.filename))
    .map((row) => row.filename)
}

/** Whether starting a writing test on this post has a reuse decision to make. False means there
 *  is nothing to reuse, so the run would observe everything anyway and the picker would only be
 *  a confirmation of the one available answer. */
export function needsPicker(
  images: readonly PostImage[],
  observations: readonly Observation[],
  videos: readonly PostVideo[] = [],
): boolean {
  if (images.length === 0 && videos.length === 0) return false
  return reobserveRows(images, observations, videos).some((row) => !row.forced)
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
