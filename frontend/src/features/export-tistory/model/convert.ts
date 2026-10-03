import type { PostImage } from '@/entities/image'
import { BlockType, GalleryLayout, type ContentLanguage, type PostContent } from '@/shared/api'
import { escapeHtml, escapeHtmlComment, headingTag, walkBlocks } from '@/shared/lib'

/** HTML fragment for Tistory's HTML editor; photo URLs are deliberately left blank. */
export function toTistory(
  content: PostContent,
  images: readonly PostImage[],
  contentLanguage: ContentLanguage,
): string {
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
        const instruction =
          contentLanguage === 'en' ? 'replace src after uploading' : '업로드 후 src 교체'
        return `<figure><img src="" alt="${escapeHtml(block.alt)}" data-file="${file}"><!-- ${commentFile} ${instruction} -->${caption}</figure>`
      }
      case BlockType.GALLERY: {
        // One figure naming its layout, each photo the IMAGE case's empty-src image with its own
        // replacement comment, and the group's one caption (EXPORT-26).
        const instruction =
          contentLanguage === 'en' ? 'replace src after uploading' : '업로드 후 src 교체'
        const layout = block.layout === GalleryLayout.SLIDE ? 'slide' : 'collage'
        const photos = block.files
          .map(
            (file) =>
              `<img src="" alt="${escapeHtml(block.alt)}" data-file="${escapeHtml(file)}"><!-- ${escapeHtmlComment(file)} ${instruction} -->`,
          )
          .join('')
        const caption = block.caption ? `<figcaption>${escapeHtml(block.caption)}</figcaption>` : ''
        return `<figure data-layout="${layout}">${photos}${caption}</figure>`
      }
      case BlockType.VIDEO: {
        // The same empty-source-plus-instruction shape an image takes, for the same reason: the
        // editor has no URL to put here, and the author attaches the original by hand.
        const file = escapeHtml(block.file)
        const commentFile = escapeHtmlComment(block.file)
        const instruction =
          contentLanguage === 'en' ? 'attach the video after uploading' : '업로드 후 영상 첨부'
        return `<video controls data-file="${file}"></video><!-- ${commentFile} ${instruction} -->`
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
