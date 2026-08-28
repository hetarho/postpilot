import type { PostImage } from '@/entities/image'
import { BlockType, type PostContent } from '@/shared/api'
import { escapeHtml, escapeHtmlComment, headingTag, walkBlocks } from '@/shared/lib'

/** HTML fragment for Tistory's HTML editor; photo URLs are deliberately left blank. */
export function toTistory(content: PostContent, images: readonly PostImage[]): string {
  // Never leak the attachment objects' expiring view URLs into the fragment.
  void images
  const blocks = walkBlocks(content, (block) => {
    switch (block.type) {
      case BlockType.TEXT:
        return `<p>${escapeHtml(block.content)}</p>`
      case BlockType.HEADING: {
        const tag = headingTag(block.level)
        return `<${tag}>${escapeHtml(block.content)}</${tag}>`
      }
      case BlockType.IMAGE: {
        const file = escapeHtml(block.file)
        const caption = block.caption ? `<figcaption>${escapeHtml(block.caption)}</figcaption>` : ''
        const commentFile = escapeHtmlComment(block.file)
        return `<figure><img src="" alt="${escapeHtml(block.alt)}" data-file="${file}"><!-- ${commentFile} 업로드 후 src 교체 -->${caption}</figure>`
      }
      case BlockType.QUOTE:
        return `<blockquote><p>${escapeHtml(block.content)}</p></blockquote>`
      case BlockType.LIST:
        return `<ul>${block.items.map((item) => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`
      default:
        return ''
    }
  }).filter(Boolean)

  return [`<p class="summary">${escapeHtml(content.summary)}</p>`, ...blocks].join('\n')
}
