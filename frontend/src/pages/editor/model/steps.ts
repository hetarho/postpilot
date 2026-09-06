import i18next from 'i18next'
import type { PostStatus } from '@/entities/post'

export type EditorStep = 'generate' | 'refine' | 'finish'

/** The post's lifecycle as three steps. One list, so the bar and the panels cannot drift. */
export function editorSteps(): ReadonlyArray<{ value: EditorStep; label: string }> {
  return [
    { value: 'generate', label: i18next.t('editor.steps.generate', { ns: 'posts' }) },
    { value: 'refine', label: i18next.t('editor.steps.refine', { ns: 'posts' }) },
    { value: 'finish', label: i18next.t('editor.steps.finish', { ns: 'posts' }) },
  ]
}

export function editorStepLabel(step: EditorStep): string {
  return editorSteps().find((item) => item.value === step)?.label ?? ''
}

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

/** Which observation the running job is on right now.
 *
 *  The stage is one word for both halves — the photo batches and then one call per clip
 *  (VIDEO-8) — so the SCREEN is what tells them apart, from two numbers it already has: how far
 *  the stage has got, and how many photos the post carries. Once `done` has passed the photos,
 *  every remaining call is a video's (VIDEO-18).
 *
 *  A post with no video always reads 사진 관찰 중, whatever the counts say. */
export function observingVideo(
  job: { stage: string; progressDone: number; progressTotal: number } | undefined,
  photoCount: number,
  videoCount: number,
): boolean {
  if (!job || job.stage !== 'observe' || videoCount === 0) return false
  return job.progressDone >= photoCount
}
