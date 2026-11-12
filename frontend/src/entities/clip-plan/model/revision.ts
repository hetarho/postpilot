/** Which document a stored revision request was about (CLIP-131). A request sent from ② names
 *  none and is `auto`: its first writing call decides whether the footage flow changes. `flow`,
 *  `narration` and `both` are the targets requests named before that and stay readable in the
 *  record (CLIP-133). */
export const CLIP_REVISION_TARGETS = ['auto', 'flow', 'narration', 'both'] as const
export type ClipRevisionTarget = (typeof CLIP_REVISION_TARGETS)[number]

/** One accepted request kept with the project, verbatim (CLIP-133): the
 *  instruction a generation froze, or the words of a revision and the document
 *  it named. An empty `body` under `instruction` is the record of a clip written
 *  without one — the absence is itself the answer. */
export interface ClipProjectRequest {
  /** `storyline` is a storyline request's words (CLIP-181). */
  kind: 'instruction' | 'storyline' | `revision:${ClipRevisionTarget}`
  body: string
  createdAt: string
}
export function isClipRequestKind(kind: string): kind is ClipProjectRequest['kind'] {
  return (
    kind === 'instruction' ||
    kind === 'storyline' ||
    CLIP_REVISION_TARGETS.some((target) => kind === `revision:${target}`)
  )
}
