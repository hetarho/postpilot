import { expect, it } from 'vitest'
import {
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import { create } from '@bufbuild/protobuf'
import { BlockSchema, BlockType } from '@/shared/api'
import { naverPhotoOrder, naverVideoOrder, toNaver } from './convert'

it('converts every block to the Naver plain-text contract', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  expect(output).toMatchSnapshot()
  expect(output).toContain('[사진 IMG_1.jpg: 비 뒤의 바다]')
  expect(output).toContain('[사진 IMG_2.jpg]')
  expect(output).not.toMatch(/<\/?(?:p|h[1-6]|img|figure|blockquote|ul|li)\b/i)
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
})

it('uses the content provenance for app-owned English photo markers', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'en')

  expect(output).toContain('[Photo IMG_1.jpg: 비 뒤의 바다]')
  expect(output).not.toContain('[사진')
})

// The strip beside the text and the markers inside it are derived from the same block array, so
// this asserts the two AGAINST EACH OTHER rather than against a hand-written list.
it('reports the photo filenames in the same order as the markers it produces', () => {
  const markers = [
    ...toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko').matchAll(/\[사진 ([^\]:]+)/g),
  ].map((match) => match[1])

  expect(naverPhotoOrder(POST_CONTENT_FIXTURE)).toEqual(markers)
})

// `toNaver` writes `[사진 ]` for an image block with no file, so the strip has to keep an entry
// for it: dropping it would shift every later photo against its own marker, which is the one
// thing this pairing exists to prevent.
it('keeps an entry for an image block with no file, because the marker keeps one too', () => {
  const content = {
    ...POST_CONTENT_FIXTURE,
    blocks: [create(BlockSchema, { type: BlockType.IMAGE }), ...POST_CONTENT_FIXTURE.blocks],
  }

  const order = naverPhotoOrder(content)
  expect(order).toEqual(['', ...naverPhotoOrder(POST_CONTENT_FIXTURE)])
  expect(order).toHaveLength(toNaver(content, POST_IMAGES_FIXTURE, 'ko').split('[사진').length - 1)
})

it('ignores every non-image block', () => {
  expect(
    naverPhotoOrder({
      blocks: POST_CONTENT_FIXTURE.blocks.filter((block) => block.type !== BlockType.IMAGE),
    }),
  ).toEqual([])
})

// VIDEO-14/15: a marker, never a URL and never bytes — the clipboard cannot carry a video file
// from a page, and the file the author filmed is on the device they are pasting from.
it('writes a video marker with the caption after a dash, and reports the video order', () => {
  const output = toNaver(POST_CONTENT_WITH_VIDEO_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  expect(output).toMatchSnapshot()
  expect(output).toContain('[동영상 clip.mp4 — 파도가 밀려온다]')
  expect(output).toContain('[동영상 clip.mp4]')
  expect(output).not.toContain('storage.example')
  expect(naverVideoOrder(POST_CONTENT_WITH_VIDEO_FIXTURE)).toEqual(['clip.mp4', 'clip.mp4'])
})

it('uses the content provenance for the English video marker too', () => {
  const output = toNaver(POST_CONTENT_WITH_VIDEO_FIXTURE, POST_IMAGES_FIXTURE, 'en')
  expect(output).toContain('[Video clip.mp4 — 파도가 밀려온다]')
  expect(output).not.toContain('[동영상')
})
