import type { PostListItem, PostStatus } from '@/entities/post'

/** The narrowing the list screen holds, straight out of the URL (POST-67). */
export interface PostNarrowing {
  q?: string
  status?: PostStatus
}

/** Trimmed, case-folded, whitespace-collapsed, and a leading `#` dropped — someone who types
 *  `#제주` is naming a tag, not searching for a hash (POST-65). The server narrows with the same
 *  steps (backend/internal/post/list.go `normalizeListText`); the two must agree, because this one
 *  names on a row the tags the server's match kept it for. */
function normalize(value: string): string {
  return value.trim().replace(/^#+/, '').replace(/\s+/g, ' ').toLocaleLowerCase()
}

/** The tags of a row that the search matched, for naming on the row while it matches (POST-65).
 *  The server does the narrowing over every owned post (POST-91); a row kept by a word its title
 *  never shows would look arbitrary without them. None while nothing is searched. */
export function matchedTags(post: PostListItem, q: string | undefined): string[] {
  const query = normalize(q ?? '')
  if (query === '') return []
  return post.tags.filter((tag) => normalize(tag).includes(query))
}
