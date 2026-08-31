import { expect, it } from 'vitest'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { toNaver } from './convert'

it('converts every block to the Naver plain-text contract', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  expect(output).toMatchSnapshot()
  expect(output).toContain('[사진 IMG_1.jpg — 비 뒤의 바다]')
  expect(output).toContain('[사진 IMG_2.jpg]')
  expect(output).not.toMatch(/<\/?(?:p|h[1-6]|img|figure|blockquote|ul|li)\b/i)
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
})

it('uses the content provenance for app-owned English photo markers', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'en')

  expect(output).toContain('[Photo IMG_1.jpg — 비 뒤의 바다]')
  expect(output).not.toContain('[사진')
})
