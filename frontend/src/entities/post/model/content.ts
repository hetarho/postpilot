import type { PostImage } from '@/entities/image/@x/post'
import type { Block } from '@/shared/api'
import type { PostDraft } from './types'

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
