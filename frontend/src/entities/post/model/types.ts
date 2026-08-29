import type { PostImage } from '@/entities/image/@x/post'
import type { GenerationJob } from '@/entities/generation-job/@x/post'
import type { Observation, PostContent } from '@/shared/api'

/** A post as the app talks about it.
 *
 *  Deliberately not the generated `Post` message: the screens should speak the app's
 *  vocabulary, so a proto change is absorbed by this entity's api mappers instead of
 *  rippling into every consumer. */
export interface PostDraft {
  slug: string
  title: string
  memo: string
  status: string
  createdAt: string
  updatedAt: string
  images: PostImage[]
  activeJob: GenerationJob | undefined
  content: PostContent | undefined
  observations: Observation[]
  pendingExperimentId: string
}

/** One row of the post list (PRD F-8). */
export interface PostListItem {
  slug: string
  title: string
  status: string
  updatedAt: string
  activeJob: GenerationJob | undefined
  pendingExperimentId: string
}

/** Shown in place of a title nobody has typed yet. A list of blank rows would be
 *  unusable, and a draft is created by typing a memo just as often as a title. */
export const UNTITLED_TITLE = '제목 없음'

/** `draft` and `review` are the statuses the drafting context knows
 *  (spec/policy/posts.md); generation is what moves a post to `review`. An unknown value
 *  falls through to itself rather than being hidden, so a status a later plan adds shows
 *  up as something rather than as a blank badge. */
const STATUS_LABELS: Record<string, string> = {
  draft: '초안',
  review: '검토',
}

export function postStatusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

export function displayTitle(post: { title: string }): string {
  return post.title.trim() || UNTITLED_TITLE
}
