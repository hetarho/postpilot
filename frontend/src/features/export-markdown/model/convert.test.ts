import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { toMarkdown } from './convert'

it('converts every block to Markdown with YAML front matter', () => {
  const output = toMarkdown(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, '2026-08-29T03:04:05Z', 'ko')

  expect(output).toMatchSnapshot()
  expect(output).toMatch(
    /^---\ntitle: .+\ndate: 2026-08-29\nlanguage: ko\nsummary: .+\ntags: \[.+\]\n---/,
  )
  expect(output).toContain('### 챙긴 것')
  expect(output).toContain('![구름 사이 햇빛](IMG_2.jpg)')
  expect(output).not.toContain('api.postpilot')
  expect(output).not.toContain('r2.cloudflarestorage')
})

it('escapes image labels and URL-encodes accepted filenames', () => {
  const content = create(PostContentSchema, {
    blocks: [
      create(BlockSchema, {
        type: BlockType.IMAGE,
        file: 'trip photo (2).jpg',
        alt: '여행 [둘]',
      }),
    ],
  })

  expect(toMarkdown(content, [], '2026-08-29', 'ko')).toContain(
    '![여행 \\[둘\\]](trip%20photo%20%282%29.jpg)',
  )
})

it('writes the concrete English content provenance to front matter', () => {
  const output = toMarkdown(POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE, '2026-08-29', 'en')

  expect(output).toContain('\nlanguage: en\n')
})
