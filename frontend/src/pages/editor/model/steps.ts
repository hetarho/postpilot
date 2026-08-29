import type { PostStatus } from '@/entities/post'

export type EditorStep = 'generate' | 'refine' | 'finish'

/** The post's lifecycle as three steps. One list, so the bar and the panels cannot drift. */
export const EDITOR_STEPS: ReadonlyArray<{ value: EditorStep; label: string }> = [
  { value: 'generate', label: '글 생성' },
  { value: 'refine', label: '글 다듬기' },
  { value: 'finish', label: '글 완성' },
]

const BY_STATUS: Record<PostStatus, EditorStep> = {
  draft: 'generate',
  review: 'refine',
  finalized: 'finish',
}

/** The step a post's status puts it in. The status IS the state — nothing new is persisted, so a
 *  reload, the list badge, and this screen cannot disagree.
 *
 *  Falls back to the first step for a status a later plan adds: 글 생성 is the step that owns the
 *  post's own fields, so an unknown status lands somewhere usable rather than on an empty panel. */
export function stepForStatus(status: string): EditorStep {
  return BY_STATUS[status as PostStatus] ?? 'generate'
}
