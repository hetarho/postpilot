import type { ClipStoryline, ClipStorylineParagraph } from '@/entities/clip-project'
import type { ClipObservations } from '@/entities/clip-observation'

/** The owner's clip storyline edits (CLIP-178), as pure functions over the whole paragraph list:
 *  the list is what an edit saves and the server validates. None adds or removes a paragraph — the
 *  count is the storyline call's. */

export function withText(
  paragraphs: readonly ClipStorylineParagraph[],
  index: number,
  text: string,
): ClipStorylineParagraph[] {
  return paragraphs.map((paragraph, at) => (at === index ? { ...paragraph, text } : paragraph))
}

/** Takes `scene` out of whichever paragraph holds it. */
export function withoutScene(
  paragraphs: readonly ClipStorylineParagraph[],
  scene: string,
): ClipStorylineParagraph[] {
  return paragraphs.map((paragraph) =>
    paragraph.observationIds.includes(scene)
      ? { ...paragraph, observationIds: paragraph.observationIds.filter((id) => id !== scene) }
      : paragraph,
  )
}

/** Puts `scene` at the end of paragraph `to`, taking it out of wherever it was — a move and a put
 *  back are the same edit. Moving a scene to the paragraph that holds it changes nothing. */
export function withSceneIn(
  paragraphs: readonly ClipStorylineParagraph[],
  scene: string,
  to: number,
): ClipStorylineParagraph[] {
  if (paragraphs[to]?.observationIds.includes(scene)) return [...paragraphs]
  return withoutScene(paragraphs, scene).map((paragraph, at) =>
    at === to ? { ...paragraph, observationIds: [...paragraph.observationIds, scene] } : paragraph,
  )
}

/** The scenes the storyline was made with that no paragraph of `paragraphs` holds — the ones the
 *  owner can put back: what the server says is out, plus what this edit took out. */
export function takenOutScenes(
  server: Pick<ClipStoryline, 'paragraphs' | 'takenOutObservationIds'>,
  paragraphs: readonly ClipStorylineParagraph[],
): string[] {
  const held = new Set(paragraphs.flatMap((paragraph) => paragraph.observationIds))
  const candidates = [
    ...server.paragraphs.flatMap((paragraph) => paragraph.observationIds),
    ...server.takenOutObservationIds,
  ]
  return [...new Set(candidates)].filter((scene) => !held.has(scene))
}

/** What a scene tile shows: its number across the project's observations, its event, and where
 *  its footage starts in which source. */
export interface ClipScene {
  id: string
  number: number
  event: string
  startMs: number
  fingerprint?: string
}

/** Every observed scene by its observation id (`sourceId/index`), numbered in the order the
 *  analysis lists them. */
export function clipScenes(
  observations: ClipObservations | undefined,
): ReadonlyMap<string, ClipScene> {
  const out = new Map<string, ClipScene>()
  for (const source of observations?.sources ?? []) {
    source.segments.forEach((segment, index) => {
      const id = `${source.source.id}/${index}`
      out.set(id, {
        id,
        number: out.size + 1,
        event: segment.event,
        startMs: segment.startMs,
        fingerprint: source.source.fingerprint,
      })
    })
  }
  return out
}
