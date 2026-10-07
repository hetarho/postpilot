import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockType, PostContentSchema } from '@/shared/api'
import { postFormReference } from './post-form-reference'
it('projects only explicitly selected title/prose/attachment positions without file/caption/tag/private evidence', () => {
  const content = create(PostContentSchema, {
    title: '선택한 제목',
    summary: 'private summary',
    tags: ['private tag'],
    blocks: [
      { type: BlockType.TEXT, content: '선택한 본문' },
      { type: BlockType.LIST, items: ['순서 1', '순서 2'] },
      {
        type: BlockType.IMAGE,
        file: 'private.jpg',
        caption: 'private caption',
        alt: 'private alt',
      },
      { type: BlockType.GALLERY, files: ['a.jpg', 'b.jpg'], caption: 'private gallery' },
      { type: BlockType.VIDEO, file: 'private.mp4', caption: 'private video' },
    ],
  })
  expect(postFormReference(content)).toBe(
    '선택한 제목\n\n선택한 본문\n\n순서 1\n순서 2\n\n[photo position]\n\n[photo group position]\n\n[video position]',
  )
})
