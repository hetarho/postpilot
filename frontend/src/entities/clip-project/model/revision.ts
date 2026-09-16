/** Which document one owner-written revision is about (CLIP-131). `narration`
 *  leaves the footage exactly where the owner put it and rewrites only what is
 *  said; the other two rewrite the flow and then the narration over it. */
export const CLIP_REVISION_TARGETS = ['flow', 'narration', 'both'] as const
export type ClipRevisionTarget = (typeof CLIP_REVISION_TARGETS)[number]

/** One accepted request kept with the project, verbatim (CLIP-133): the
 *  instruction a generation froze, or the words of a revision and the document
 *  it named. An empty `body` under `instruction` is the record of a clip written
 *  without one — the absence is itself the answer. */
export interface ClipProjectRequest {
  kind: 'instruction' | `revision:${ClipRevisionTarget}`
  body: string
  createdAt: string
}
export function isClipRequestKind(kind: string): kind is ClipProjectRequest['kind'] {
  return (
    kind === 'instruction' || CLIP_REVISION_TARGETS.some((target) => kind === `revision:${target}`)
  )
}
