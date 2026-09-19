import { clone, create } from '@bufbuild/protobuf'
import type { PostImage } from '@/entities/image/@x/post'
import {
  BlockSchema,
  PostContentSchema,
  type Block,
  type BlockType,
  type PostContent,
} from '@/shared/api'
import type { PostDraft } from './types'

type PostContentPatch = Partial<Omit<PostContent, '$typeName' | '$unknown'>>
type BlockPatch = Partial<Omit<Block, '$typeName' | '$unknown'>>

/** The canonical post is a block array carried as a protobuf message (I2), so every edit produces
 *  a NEW message rather than mutating the cached one. These four constructors are the whole
 *  vocabulary an editor needs, and they keep the message schemas inside the entity (ARCH-17). */
export function copyPostContent(content: PostContent): PostContent {
  return clone(PostContentSchema, content)
}
export function postContentWith(content: PostContent, patch: PostContentPatch): PostContent {
  return create(PostContentSchema, { ...content, ...patch })
}
export function blockWith(block: Block, patch: BlockPatch): Block {
  return create(BlockSchema, { ...block, ...patch })
}
export function newBlock(init: BlockPatch & { type: BlockType }): Block {
  return create(BlockSchema, init)
}

/** Exact filename lookup shared by block rendering and exporters. */
export function imageByFile(images: readonly PostImage[]): ReadonlyMap<string, PostImage> {
  return new Map(images.map((image) => [image.filename, image]))
}

/** Stable enough for a read-only model result while still disambiguating repeats. */
export function blockKey(block: Block, index: number): string {
  return `${block.type}:${block.file || block.content || block.items.join('\u001f')}:${index}`
}

export function hasContent(post: Pick<PostDraft, 'content'>): boolean {
  return post.content !== undefined
}
