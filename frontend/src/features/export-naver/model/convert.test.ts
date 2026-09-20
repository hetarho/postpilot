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
  // The caption rides INSIDE the marker, folded; the photo with none keeps the bare form.
  expect(output).toContain('사진_1_비_뒤의_바다_사진')
  expect(output).toContain('사진_2_사진')
  expect(output).not.toMatch(/<\/?(?:p|h[1-6]|img|figure|blockquote|ul|li)\b/i)
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
})

// EXPORT-5: the filename is not something to paste, and the caption travels FOLDED — a space
// inside the marker is exactly where a double-click's selection would stop.
it('carries the folded caption and no filename in a photo marker', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko')

  expect(output).not.toContain('IMG_1.jpg')
  expect(output).not.toContain('IMG_2.jpg')
  expect(output).not.toContain('비 뒤의 바다')
  expect(output).not.toMatch(/\[사진/)
})

// The whole of the fold, stated: every run of whitespace, punctuation and symbols becomes one
// `_`, nothing else is touched, and a caption that folds to nothing leaves the bare marker.
it('folds a caption into one double-clickable token', () => {
  for (const [caption, want] of [
    ['비 뒤의 바다', '사진_1_비_뒤의_바다_사진'],
    ['맥북(M4)', '사진_1_맥북_M4_사진'],
    ['  앞뒤 공백  ', '사진_1_앞뒤_공백_사진'],
    ['쉼표, 그리고  두 칸', '사진_1_쉼표_그리고_두_칸_사진'],
    ['이미_스네이크', '사진_1_이미_스네이크_사진'],
    ['🙂', '사진_1_사진'],
    ['', '사진_1_사진'],
  ] as const) {
    const content = {
      ...POST_CONTENT_FIXTURE,
      blocks: [create(BlockSchema, { type: BlockType.IMAGE, file: 'IMG_1.jpg', caption })],
    }
    expect(toNaver(content, POST_IMAGES_FIXTURE, 'ko')).toBe(want)
  }
})

it('uses the content provenance for app-owned English photo markers', () => {
  const output = toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'en')

  expect(output).toContain('photo_1_비_뒤의_바다_photo')
  expect(output).toContain('photo_2_photo')
  expect(output).not.toContain('사진_')
})

// The preview beside the text and the markers inside it are derived from the same block array,
// so this asserts the two AGAINST EACH OTHER: the marker numbers run 1..n in the order
// `naverPhotoOrder` reports, which is the pairing the whole strip depends on.
it('numbers the markers from 1 in the order it reports the photos', () => {
  const numbers = [
    ...toNaver(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, 'ko').matchAll(/사진_(\d+)_[^\n]*사진/g),
  ].map((match) => Number(match[1]))

  expect(numbers).toEqual(naverPhotoOrder(POST_CONTENT_FIXTURE).map((_, index) => index + 1))
})

// `toNaver` still spends a number on an image block with no file, so the strip has to keep an
// entry for it: dropping either one would shift every later photo against its own marker, which
// is the one thing this pairing exists to prevent.
it('keeps an entry for an image block with no file, because the marker spends its number too', () => {
  const content = {
    ...POST_CONTENT_FIXTURE,
    blocks: [create(BlockSchema, { type: BlockType.IMAGE }), ...POST_CONTENT_FIXTURE.blocks],
  }

  const order = naverPhotoOrder(content)
  const output = toNaver(content, POST_IMAGES_FIXTURE, 'ko')
  expect(order).toEqual(['', ...naverPhotoOrder(POST_CONTENT_FIXTURE)])
  expect(output.match(/사진_\d+_[^\n]*사진/g)).toHaveLength(order.length)
  // The hole is the FIRST marker and it has no caption, so the captioned photo that was
  // 사진_1_… is now 사진_2_….
  expect(output).toContain('사진_1_사진')
  expect(output).toContain('사진_2_비_뒤의_바다_사진')
  expect(output).toContain('사진_3_사진')
})

// EXPORT-4: the slot placeholder keeps its brackets and the photo marker has none, which is now
// the whole of what tells them apart in the pasted body.
it('keeps an unfilled slot bracketed and unreadable as a photo marker', () => {
  const content = {
    ...POST_CONTENT_FIXTURE,
    blocks: [
      create(BlockSchema, { type: BlockType.TEXT, slot: { kind: 'place', label: '네이버 지도' } }),
      ...POST_CONTENT_FIXTURE.blocks,
    ],
  }

  const output = toNaver(content, POST_IMAGES_FIXTURE, 'ko')
  expect(output).toContain('[네이버 지도]')
  expect(output).not.toMatch(/\[[^\]]*사진_\d+_/)
  expect(output.match(/사진_\d+_[^\n]*사진/g)).toHaveLength(naverPhotoOrder(content).length)
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
