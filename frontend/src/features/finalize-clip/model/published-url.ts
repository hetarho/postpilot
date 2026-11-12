import type { ClipChannel } from '@/entities/clip-project'

/** The longest posted URL the server stores (CLIP-232). */
export const CLIP_PUBLISHED_URL_MAX_CHARS = 2048

const HOSTS: Record<ClipChannel, readonly string[]> = {
  naver_clip: ['naver.com', 'naver.me'],
  instagram_clip: ['instagram.com'],
}

/** An `https` URL on the clip channel's platform host or one of its subdomains, checked before
 *  any request in the server's own words; the server reads the same rule and stays
 *  authoritative (CLIP-232). Nothing is fetched. */
export function validClipPublishedUrl(channel: ClipChannel, value: string): boolean {
  if (value.length > CLIP_PUBLISHED_URL_MAX_CHARS) return false
  let url: URL
  try {
    url = new URL(value)
  } catch {
    return false
  }
  if (url.protocol !== 'https:' || url.username || url.password) return false
  const host = url.hostname.toLowerCase()
  return HOSTS[channel].some((root) => host === root || host.endsWith(`.${root}`))
}
