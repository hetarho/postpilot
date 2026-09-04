import type { PostImage } from '@/entities/image'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import {
  blockSlotPlaceholder,
  escapeHtml,
  escapeMarkdownLabel,
  relativeFileUrl,
  walkBlocks,
  yamlString,
} from '@/shared/lib'

/** Markdown with YAML front matter and relative filename image references. */
export function toMarkdown(
  content: PostContent,
  _images: readonly PostImage[],
  createdAt: string,
  contentLanguage: ContentLanguage,
): string {
  const date = createdAt.match(/^\d{4}-\d{2}-\d{2}/)?.[0] ?? ''
  const frontMatter = [
    '---',
    `title: ${yamlString(content.title)}`,
    `date: ${date}`,
    `language: ${contentLanguage}`,
    `summary: ${yamlString(content.summary)}`,
    `tags: [${content.tags.map(yamlString).join(', ')}]`,
    '---',
  ].join('\n')
  const body = walkBlocks(content, (block) => {
    switch (block.type) {
      case BlockType.TEXT: {
        const slot = blockSlotPlaceholder(block)
        return escapeHtml(slot ?? block.content)
      }
      case BlockType.HEADING:
        return `${block.level === 3 ? '###' : '##'} ${escapeHtml(block.content)}`
      case BlockType.IMAGE:
        return [
          `![${escapeMarkdownLabel(block.alt)}](${relativeFileUrl(block.file)})`,
          block.caption ? `*${escapeHtml(block.caption)}*` : '',
        ]
          .filter(Boolean)
          .join('\n')
      case BlockType.QUOTE:
        return `> ${escapeHtml(block.content)}`
      case BlockType.LIST:
        return block.items.map((item) => `- ${escapeHtml(item)}`).join('\n')
      default:
        return ''
    }
  })
    .filter(Boolean)
    .join('\n\n')

  return `${frontMatter}\n\n${body}`
}
