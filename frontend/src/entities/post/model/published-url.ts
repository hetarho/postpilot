import { POST_PUBLISHED_URL_MAX_CHARS } from '../config'

/** scheme://authority path ?query #fragment. `new URL()` is not used: it silently drops a
 *  default port such as `:443`, which the server refuses, so the two would disagree. */
const ADDRESS = /^([a-z][a-z0-9+.-]*):\/\/([^/?#]*)([^?#]*)(?:\?([^#]*))?(?:#.*)?$/i

const NAVER_BLOG_HOSTS = new Set(['blog.naver.com', 'm.blog.naver.com'])

/** The Naver Blog address rule (POST-77), the browser's copy of the server's: the stored form of
 *  an accepted address, or undefined for a refused one. The shared case file pins the two
 *  together (`backend/internal/post/testdata/published_url/cases.json`).
 *
 *  It only gates the request. What is sent is the trimmed input, and what is shown is the
 *  server's answer, so this never becomes a second normalization of record. */
export function parseNaverBlogUrl(raw: string): string | undefined {
  const value = raw.trim()
  if (value === '' || Array.from(value).length > POST_PUBLISHED_URL_MAX_CHARS) return undefined
  const match = ADDRESS.exec(value)
  if (!match) return undefined
  const [, scheme, authority, path, query] = match
  const lowered = scheme.toLowerCase()
  if (lowered !== 'http' && lowered !== 'https') return undefined
  // Userinfo, and a port — an empty one included.
  if (authority.includes('@') || authority.includes(':')) return undefined
  if (!NAVER_BLOG_HOSTS.has(authority.toLowerCase())) return undefined
  // A path of nothing but slashes is the blog's home, not a post.
  if (path.replace(/^\/+|\/+$/g, '') === '') return undefined
  return `https://blog.naver.com${path}${query ? `?${query}` : ''}`
}
