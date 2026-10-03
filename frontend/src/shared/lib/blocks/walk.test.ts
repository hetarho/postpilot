import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, BlockType, GalleryLayout } from '@/shared/api'
import { blockPhotos } from './walk'

it('names the photos a block holds, in order', () => {
  expect(blockPhotos(create(BlockSchema, { type: BlockType.IMAGE, file: 'a.jpg' }))).toEqual([
    'a.jpg',
  ])
  // An IMAGE block with no file still holds a place: the Naver markers spend a number on it.
  expect(blockPhotos(create(BlockSchema, { type: BlockType.IMAGE }))).toEqual([''])
  expect(
    blockPhotos(
      create(BlockSchema, {
        type: BlockType.GALLERY,
        files: ['b.jpg', 'c.jpg'],
        layout: GalleryLayout.SLIDE,
      }),
    ),
  ).toEqual(['b.jpg', 'c.jpg'])
  expect(blockPhotos(create(BlockSchema, { type: BlockType.VIDEO, file: 'clip.mp4' }))).toEqual([])
  expect(blockPhotos(create(BlockSchema, { type: BlockType.TEXT, content: '문단' }))).toEqual([])
})
