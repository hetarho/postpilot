/** Which document one owner-written revision is about (CLIP-131). `narration`
 *  leaves the footage exactly where the owner put it and rewrites only what is
 *  said; the other two rewrite the flow and then the narration over it. */
export const CLIP_REVISION_TARGETS = ['flow', 'narration', 'both'] as const
export type ClipRevisionTarget = (typeof CLIP_REVISION_TARGETS)[number]
