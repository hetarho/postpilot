import { expect, it } from 'vitest'
import { CLIP_PUBLISHED_URL_MAX_CHARS, validClipPublishedUrl } from './published-url'

it('accepts https on the channel’s platform host and its subdomains', () => {
  expect(validClipPublishedUrl('naver_clip', 'https://clip.naver.com/abc')).toBe(true)
  expect(validClipPublishedUrl('naver_clip', 'https://naver.me/xyz')).toBe(true)
  expect(validClipPublishedUrl('instagram_clip', 'https://www.instagram.com/reel/1')).toBe(true)
})

it('refuses another channel’s host, plain http, look-alike hosts and overlong values', () => {
  expect(validClipPublishedUrl('naver_clip', 'https://www.instagram.com/reel/1')).toBe(false)
  expect(validClipPublishedUrl('instagram_clip', 'http://www.instagram.com/reel/1')).toBe(false)
  expect(validClipPublishedUrl('naver_clip', 'https://evilnaver.com/a')).toBe(false)
  expect(validClipPublishedUrl('naver_clip', 'not a url')).toBe(false)
  expect(
    validClipPublishedUrl(
      'naver_clip',
      `https://clip.naver.com/${'a'.repeat(CLIP_PUBLISHED_URL_MAX_CHARS)}`,
    ),
  ).toBe(false)
})
