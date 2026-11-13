import i18next from 'i18next'
import { isTerminal } from '@/entities/generation-job/@x/post'
import { isPostStatus, postStatusLabel, type PostListItem, type PostStatus } from './types'

/** The badge tones a row state reads in; the label carries the meaning on its own and the tone
 *  only reinforces it (THEME-18). */
export type PostRowTone = 'neutral' | 'accent' | 'success' | 'done' | 'info' | 'danger' | 'warning'

/** 초안 · 검토 · 확정 sat in one grey and were indistinguishable at a glance. Draft stays neutral
 *  (nothing has happened yet), review takes the accent (the user is mid-way), finalized takes
 *  the tinted success (done writing). Published in the same tint read as 확정 at a glance, so the
 *  post that is finished for good takes the solid `done` plane (THEME-18, THEME-29). */
const STATUS_TONE: Record<PostStatus, PostRowTone> = {
  draft: 'neutral',
  review: 'accent',
  finalized: 'success',
  published: 'done',
}

type RowPost = Pick<
  PostListItem,
  | 'status'
  | 'activeJob'
  | 'latestOrdinaryFailure'
  | 'pendingExperimentId'
  | 'contentReady'
  | 'storylineReady'
>

/** The one status chip a post's row carries, in the blog list and in aggregate 내 글 alike, so
 *  the two cannot drift. A running job, then a failure, then a pending test's result outrank the
 *  post's own state; a draft holding a storyline and no content reads 스토리라인 작성 완료 as a
 *  mid-way state, the name a clip gives the same stage (POST-127 →CLIP-226). These are derived
 *  row states, never statuses: the status filter keeps reading `status` alone (POST-66).
 *  `pendingExperimentStatus` is the pending test's own status where the caller reads it. */
export function postRowState(
  post: RowPost,
  pendingExperimentStatus?: string,
): { label: string; tone: PostRowTone } {
  if (post.activeJob && !isTerminal(post.activeJob))
    return { label: rowStateLabel('generating'), tone: 'info' }
  if (post.latestOrdinaryFailure || post.activeJob?.status === 'failed')
    return { label: rowStateLabel('failed'), tone: 'danger' }
  if (pendingExperimentStatus === 'failed' || pendingExperimentStatus === 'partial')
    return { label: rowStateLabel('failed'), tone: 'danger' }
  if (post.pendingExperimentId) return { label: rowStateLabel('review'), tone: 'warning' }
  if (post.status === 'draft' && post.storylineReady && !post.contentReady)
    return { label: rowStateLabel('storylineReady'), tone: 'accent' }
  return {
    label: postStatusLabel(post.status),
    tone: isPostStatus(post.status) ? STATUS_TONE[post.status] : 'neutral',
  }
}

function rowStateLabel(state: 'generating' | 'failed' | 'review' | 'storylineReady'): string {
  return i18next.t(`rowState.${state}`, { ns: 'posts' })
}
