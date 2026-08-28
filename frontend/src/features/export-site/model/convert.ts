import type { PostImage } from '@/entities/image'
import { BlockType, type PostContent } from '@/shared/api'
import { escapeHtml, headingTag, relativeFileUrl, walkBlocks } from '@/shared/lib'
import { SITE_DOCUMENT_PREFIX, SITE_DOCUMENT_SUFFIX, SITE_STYLE } from '../config/template'

/** A complete static page whose CSS and shell never depend on model output. */
export function toSite(
  content: PostContent,
  _images: readonly PostImage[],
  createdAt: string,
): string {
  const date = createdAt.match(/^\d{4}-\d{2}-\d{2}/)?.[0] ?? ''
  const article = walkBlocks(content, (block) => {
    switch (block.type) {
      case BlockType.TEXT:
        return `<p>${escapeHtml(block.content)}</p>`
      case BlockType.HEADING: {
        const tag = headingTag(block.level)
        return `<${tag}>${escapeHtml(block.content)}</${tag}>`
      }
      case BlockType.IMAGE: {
        const caption = block.caption ? `<figcaption>${escapeHtml(block.caption)}</figcaption>` : ''
        return `<figure><img src="${escapeHtml(relativeFileUrl(block.file))}" alt="${escapeHtml(block.alt)}">${caption}</figure>`
      }
      case BlockType.QUOTE:
        return `<blockquote><p>${escapeHtml(block.content)}</p></blockquote>`
      case BlockType.LIST:
        return `<ul>${block.items.map((item) => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`
      default:
        return ''
    }
  })
    .filter(Boolean)
    .join('\n')
  const tags = content.tags.map((tag) => `<li>${escapeHtml(tag)}</li>`).join('')

  return `${SITE_DOCUMENT_PREFIX}
<title>${escapeHtml(content.title)}</title>
<style>${SITE_STYLE}</style>
</head>
<body>
<main>
<header>
<h1>${escapeHtml(content.title)}</h1>
<p class="summary">${escapeHtml(content.summary)}</p>
<p class="meta"><time datetime="${escapeHtml(date)}">${escapeHtml(date)}</time></p>
<ul class="tags">${tags}</ul>
</header>
<article>
${article}
</article>
</main>
${SITE_DOCUMENT_SUFFIX}`
}
