import type { PostStatus } from '@/entities/post'
import { LIFECYCLE_STATIONS, lifecycleStationLabel, type LifecycleStation } from '@/shared/config'

export type EditorStep = LifecycleStation

/** The post's lifecycle as the five shared stations (POST-44, THEME-39). One list, so the bar and
 *  the panels cannot drift, and the same short names the clip workspace runs. */
export function editorSteps(): ReadonlyArray<{ value: EditorStep; label: string }> {
  return LIFECYCLE_STATIONS.map((value) => ({ value, label: lifecycleStationLabel(value) }))
}

export function editorStepLabel(step: EditorStep): string {
  return lifecycleStationLabel(step)
}

const BY_STATUS: Partial<Record<PostStatus, EditorStep>> = {
  review: 'refine',
  finalized: 'finish',
  // A published post has nothing left to write (POST-44): it opens where its address lives.
  published: 'finish',
}

/** The persisted facts the step is derived from. */
export interface PostStepFacts {
  status: string
  hasObservations: boolean
  hasStoryline: boolean
}

/** The step a post's persisted facts put it in. Nothing new is persisted, so a reload, the list
 *  badge, and this screen cannot disagree (POST-44):
 *
 *  - a draft holding a storyline opens ③ 스토리라인, where it is read and written from;
 *  - a draft holding observations only opens ② 관찰, where they are read and corrected;
 *  - any other draft opens ① 생성; review opens ④ 다듬기; finalized and published open ⑤ 완성.
 *
 *  Falls back to the first step for a status a later plan adds: 생성 is the step that owns the
 *  post's own fields, so an unknown status lands somewhere usable rather than on an empty panel. */
export function stepForPost({ status, hasObservations, hasStoryline }: PostStepFacts): EditorStep {
  if (status === 'draft') {
    if (hasStoryline) return 'storyline'
    if (hasObservations) return 'observe'
    return 'generate'
  }
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
