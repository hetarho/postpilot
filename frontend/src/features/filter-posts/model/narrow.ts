import type { PostListItem, PostStatus } from '@/entities/post'

/** The narrowing the list screen holds, straight out of the URL (POST-67). */
export interface PostNarrowing {
  q?: string
  status?: PostStatus
}

/** One kept row, plus the reason it was kept when the reason is not on screen. A row matched by
 *  a tag says so on the row (POST-65), and only the tags that actually matched: the query hit
 *  something the title never showed, so without them the row looks arbitrary. */
export interface NarrowedPost {
  post: PostListItem
  matchedTags: string[]
}

/** Trimmed, case-folded, whitespace-collapsed, and a leading `#` dropped — someone who types
 *  `#제주` is naming a tag, not searching for a hash (POST-65). */
function normalize(value: string): string {
  return value.trim().replace(/^#+/, '').replace(/\s+/g, ' ').toLocaleLowerCase()
}

/** Narrows the one list answer in the browser: the server returns every post the account owns,
 *  newest first (POST-3), and neither control is a query.
 *
 *  The search reads the title as stored and the post's tags. It deliberately does NOT read the
 *  `제목 없음` placeholder: that is the list's word for a post nobody has titled, not the post's
 *  own text, so searching for it would match rows that contain nothing of the sort. */
export function narrowPosts(posts: PostListItem[], narrowing: PostNarrowing): NarrowedPost[] {
  const query = normalize(narrowing.q ?? '')
  const kept: NarrowedPost[] = []
  for (const post of posts) {
    // `post.status` alone (POST-66): AI 생성 중 and AI 결과 확인 are things the badge says about
    // a job, not statuses, so a draft mid-generation stays under 초안.
    if (narrowing.status && post.status !== narrowing.status) continue
    if (query === '') {
      kept.push({ post, matchedTags: [] })
      continue
    }
    const matchedTags = post.tags.filter((tag) => normalize(tag).includes(query))
    if (normalize(post.title).includes(query) || matchedTags.length > 0) {
      kept.push({ post, matchedTags })
    }
  }
  return kept
}
