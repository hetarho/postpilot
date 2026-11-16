import { expect, it } from 'vitest'
import { CLIP_PUBLISHED_URL_MAX_CHARS, validClipPublishedUrl } from './published-url'

it('accepts https on the naver hosts and its subdomains', () => {
  expect(validClipPublishedUrl('https://clip.naver.com/abc')).toBe(true)
  expect(validClipPublishedUrl('https://naver.me/xyz')).toBe(true)
  expect(validClipPublishedUrl('https://m.naver.com/a')).toBe(true)
})

it('refuses another host, plain http, look-alike hosts and overlong values', () => {
  expect(validClipPublishedUrl('https://www.instagram.com/reel/1')).toBe(false)
  expect(validClipPublishedUrl('http://clip.naver.com/a')).toBe(false)
  expect(validClipPublishedUrl('https://evilnaver.com/a')).toBe(false)
  expect(validClipPublishedUrl('not a url')).toBe(false)
  expect(
    validClipPublishedUrl(`https://clip.naver.com/${'a'.repeat(CLIP_PUBLISHED_URL_MAX_CHARS)}`),
  ).toBe(false)
})
