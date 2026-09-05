import i18next from 'i18next'
import type { PostImage } from '@/entities/image/@x/post'
import type { GenerationJob } from '@/entities/generation-job/@x/post'
import type { TemplateRef } from '@/entities/template/@x/post'
import type { VoiceRef } from '@/entities/voice/@x/post'
import type { ContentLanguage, Observation, PostContent } from '@/shared/api'

/** A post as the app talks about it.
 *
 *  Deliberately not the generated `Post` message: the screens should speak the app's
 *  vocabulary, so a proto change is absorbed by this entity's api mappers instead of
 *  rippling into every consumer. */
export type PostStatus = 'draft' | 'review' | 'finalized'

export interface PostDraft {
  slug: string
  title: string
  memo: string
  status: PostStatus
  createdAt: string
  updatedAt: string
  /** The voice the post is written in. Always present — a post cannot exist without one — and
   *  still named after the voice is deleted (spec/legacy/policy/posts.md). */
  voice: VoiceRef
  /** The 템플릿 the post is written for, or an empty ref for 없음. Optional by design: unlike the
   *  voice, the server never picks one (spec/legacy/policy/templates.md). */
  template: TemplateRef
  images: PostImage[]
  activeJob: GenerationJob | undefined
  content: PostContent | undefined
  observations: Observation[]
  pendingExperimentId: string
  contentRevision: bigint
  machineBaselineRevision: bigint
  /** The voice the latest machine baseline was written under; empty when there is none. Learning
   *  is possible only while it equals `voice.id`, so a reassigned post must be regenerated first. */
  machineBaselineVoiceId: string
  canFinalize: boolean
  targetLength?: number
  finalizedRevision: bigint
  finalizedAt: string
  targetLanguage: ContentLanguage
  contentLanguage: ContentLanguage | undefined
}

/** One row of the post list (PRD F-8). */
export interface PostListItem {
  slug: string
  title: string
  status: PostStatus
  updatedAt: string
  voice: VoiceRef
  template: TemplateRef
  activeJob: GenerationJob | undefined
  pendingExperimentId: string
  targetLanguage: ContentLanguage
  contentLanguage: ContentLanguage | undefined
}

/** Shown in place of a title nobody has typed yet. A list of blank rows would be
 *  unusable, and a draft is created by typing a memo just as often as a title. */
export function untitledTitle(): string {
  return i18next.t('untitled', { ns: 'posts' })
}

/** `draft` and `review` are the statuses the drafting context knows
 *  (spec/legacy/policy/posts.md); generation is what moves a post to `review`. An unknown value
 *  falls through to itself rather than being hidden, so a status a later plan adds shows
 *  up as something rather than as a blank badge. */
export function postStatusLabel(status: string): string {
  if (status === 'draft' || status === 'review' || status === 'finalized') {
    return i18next.t(`status.${status}`, { ns: 'posts' })
  }
  return status
}

export function displayTitle(post: { title: string }): string {
  return post.title.trim() || untitledTitle()
}
