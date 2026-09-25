import i18next from 'i18next'
import type { BlogFieldChoice } from '@/entities/blog-field/@x/post'
import type { PostImage } from '@/entities/image/@x/post'
import type { PostVideo } from '@/entities/video/@x/post'
import type { GenerationJob } from '@/entities/generation-job/@x/post'
import type { QualityMetricId } from '@/entities/quality/@x/post'
import type { TemplateRef } from '@/entities/template/@x/post'
import type { VoiceRef } from '@/entities/voice/@x/post'
import type { ContentLanguage, Observation, PostContent } from '@/shared/api'
import type { ReplacementCandidate } from './replacements'

/** A post as the app talks about it.
 *
 *  Deliberately not the generated `Post` message: the screens should speak the app's
 *  vocabulary, so a proto change is absorbed by this entity's api mappers instead of
 *  rippling into every consumer. */
export type PostStatus = 'draft' | 'review' | 'finalized' | 'published'

/** Every status in lifecycle order, which is also the order the list's filter offers them. */
export const POST_STATUSES: readonly PostStatus[] = ['draft', 'review', 'finalized', 'published']

export function isPostStatus(value: unknown): value is PostStatus {
  return (POST_STATUSES as readonly unknown[]).includes(value)
}

/** The one question every published-post lock asks (POST-86): a published post takes no write
 *  but its address and the delete, so each screen that offers one reads this, not the literal. */
export function isPublished(post: { status: string }): boolean {
  return post.status === 'published'
}

/** One answer a post gives to a data field its template declared. `enabled` off means "I have
 *  nothing for this": the text is kept and the enqueue drops the whole block. */
export interface PostTemplateAnswer {
  label: string
  text: string
  enabled: boolean
}

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
  /** The post's 분야, '' for 없음 (POST-82). A number a newer server adds reads as 없음: it is
   *  not one this build can offer or show. */
  field: BlogFieldChoice
  /** What this post answers to its template's data fields, by label. ① seeds its fields from
   *  these; a label the current template does not declare is kept and never rendered
   *  (POST-62). */
  templateAnswers: PostTemplateAnswer[]
  images: PostImage[]
  /** The post's second attachment kind, ordered like the photos. Separate from `images`
   *  rather than discriminated inside it: almost every surface renders the two differently,
   *  and a client that had to read a flag to know which would get it wrong once (VIDEO-1). */
  videos: PostVideo[]
  activeJob: GenerationJob | undefined
  content: PostContent | undefined
  /** What the last write offered to replace, as stored (GEN-53). Stale ones stay stored and are
   *  dropped where they render; one with a surface this build cannot name is dropped here. */
  replacementCandidates: ReplacementCandidate[]
  observations: Observation[]
  pendingExperimentId: string
  contentRevision: bigint
  machineBaselineRevision: bigint
  /** The voice the latest machine baseline was written under; empty when there is none. Learning
   *  is possible only while it equals `voice.id`, so a reassigned post must be regenerated first. */
  machineBaselineVoiceId: string
  canFinalize: boolean
  targetLength?: number
  /** Always concrete: the server fills it and an older message falls back to the default. */
  tagCount: number
  /** Whether a run may carry the account's memories (MEM-18). Default off, and off is what every
   *  draft saved before memories existed reads as — such a post's prompt is byte-identical to the
   *  one it produced before the domain existed. */
  useMemory: boolean
  /** The quality metrics ticked for the next run, in catalogue order (POST-81). The enqueue reads
   *  them; a stored tick whose metric is no longer over band is kept and ignored there. */
  qualityRules: QualityMetricId[]
  finalizedRevision: bigint
  finalizedAt: string
  /** The Naver Blog address the post was published at, and when; both `''` until it is. */
  publishedUrl: string
  publishedAt: string
  targetLanguage: ContentLanguage
  contentLanguage: ContentLanguage | undefined
}

/** The writing brief's run options, saved together (POST-89): `targetLength` undefined is
 *  natural length, `field` '' is 없음. */
export interface GenerationOptionsSet {
  targetLength?: number
  tagCount: number
  useMemory: boolean
  qualityRules: readonly QualityMetricId[]
  field: BlogFieldChoice
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
  /** The current content revision's tags (POST-65), so the list can be narrowed by them. Empty
   *  until something has written content for the post. */
  tags: string[]
}

/** Shown in place of a title nobody has typed yet. A list of blank rows would be
 *  unusable, and a draft is created by typing a memo just as often as a title. */
export function untitledTitle(): string {
  return i18next.t('untitled', { ns: 'posts' })
}

/** The status's own word. An unknown value falls through to itself rather than being hidden, so
 *  a status a later plan adds shows up as something rather than as a blank badge. */
export function postStatusLabel(status: string): string {
  if (isPostStatus(status)) return i18next.t(`status.${status}`, { ns: 'posts' })
  return status
}

export function displayTitle(post: { title: string }): string {
  return post.title.trim() || untitledTitle()
}
