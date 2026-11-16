/** The longest posted URL the server stores (CLIP-232). */
export const CLIP_PUBLISHED_URL_MAX_CHARS = 2048

/** The one platform host list: 네이버 클립 (CLIP-209, CLIP-212). */
const CLIP_PUBLISHED_URL_HOSTS = ['naver.com', 'naver.me'] as const

/** An `https` URL on 네이버 클립's platform host or one of its subdomains, checked before any
 *  request in the server's own words; the server reads the same rule and stays authoritative
 *  (CLIP-232). Nothing is fetched. */
export function validClipPublishedUrl(value: string): boolean {
  if (value.length > CLIP_PUBLISHED_URL_MAX_CHARS) return false
  let url: URL
  try {
    url = new URL(value)
  } catch {
    return false
  }
  if (url.protocol !== 'https:' || url.username || url.password) return false
  const host = url.hostname.toLowerCase()
  return CLIP_PUBLISHED_URL_HOSTS.some((root) => host === root || host.endsWith(`.${root}`))
}
