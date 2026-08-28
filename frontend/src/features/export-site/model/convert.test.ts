import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { toSite } from './convert'

it('converts every block to one standalone fixed-template page', () => {
  const output = toSite(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, '2026-08-29T03:04:05Z')

  expect(output).toMatchSnapshot()
  expect(output.startsWith('<!doctype html>')).toBe(true)
  expect(output).toContain('<html lang="ko">')
  expect(output).toContain('src="IMG_1.jpg"')
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
})

it('uses byte-identical style content for different posts', () => {
  const first = toSite(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, '2026-08-29T03:04:05Z')
  const second = toSite(
    create(PostContentSchema, {
      title: '완전히 다른 글',
      summary: '다른 요약',
      tags: ['다름'],
      blocks: [],
    }),
    [],
    '2025-01-02T00:00:00Z',
  )
  const style = (value: string) => value.match(/<style>([\s\S]*?)<\/style>/)?.[1]

  expect(style(first)).toBe(style(second))
})

it('uses a URL-safe relative path for an accepted filename', () => {
  const content = create(PostContentSchema, {
    blocks: [
      create(BlockSchema, {
        type: BlockType.IMAGE,
        file: 'trip photo?#.jpg',
        alt: '사진',
      }),
    ],
  })

  expect(toSite(content, [], '2026-08-29')).toContain('src="trip%20photo%3F%23.jpg"')
})
