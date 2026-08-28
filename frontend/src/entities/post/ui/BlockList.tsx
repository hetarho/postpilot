import { BlockType, type PostContent } from '@/shared/api'
import { Thumbnail, type PostImage } from '@/entities/image/@x/post'
import { blockKey, imageByFile } from '../model/content'

interface BlockListProps {
  content: PostContent
  images: readonly PostImage[]
}

/** The canonical block array rendered as a read-only draft. */
export function BlockList({ content, images }: BlockListProps) {
  const imagesByFile = imageByFile(images)

  return (
    <article aria-label="생성된 글" className="mt-12 pb-12">
      <header>
        <p className="text-content-tertiary text-xs font-medium tracking-wide uppercase">Draft</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">{content.title}</h1>
        {content.summary && (
          <p className="text-content-secondary mt-3 text-sm leading-relaxed">{content.summary}</p>
        )}
        {content.tags.length > 0 && (
          <ul aria-label="태그" className="mt-3 flex flex-wrap gap-2">
            {content.tags.map((tag) => (
              <li
                key={tag}
                className="bg-surface-raised text-content-secondary rounded-sm px-2 py-1 text-xs"
              >
                #{tag}
              </li>
            ))}
          </ul>
        )}
      </header>

      <div className="mt-8 space-y-5">
        {content.blocks.map((block, index) => {
          const key = blockKey(block, index)
          switch (block.type) {
            case BlockType.TEXT:
              return (
                <p key={key} className="text-sm leading-relaxed whitespace-pre-wrap">
                  {block.content}
                </p>
              )
            case BlockType.HEADING:
              return block.level === 3 ? (
                <h3 key={key} className="pt-3 text-base font-semibold tracking-tight">
                  {block.content}
                </h3>
              ) : (
                <h2 key={key} className="pt-5 text-lg font-semibold tracking-tight">
                  {block.content}
                </h2>
              )
            case BlockType.IMAGE: {
              const image = imagesByFile.get(block.file)
              if (!image) return null
              return (
                <div key={key} className="py-2">
                  <Thumbnail src={image.viewUrl} alt={block.alt || block.file} />
                  {block.caption && (
                    <p className="text-content-tertiary mt-2 text-xs">{block.caption}</p>
                  )}
                </div>
              )
            }
            case BlockType.QUOTE:
              return (
                <blockquote
                  key={key}
                  className="bg-surface-recessed text-content-secondary rounded-md px-4 py-3 text-sm leading-relaxed"
                >
                  {block.content}
                </blockquote>
              )
            case BlockType.LIST:
              return (
                <ul key={key} className="list-disc space-y-1 pl-5 text-sm leading-relaxed">
                  {block.items.map((item, itemIndex) => (
                    <li key={`${item}:${itemIndex}`}>{item}</li>
                  ))}
                </ul>
              )
            default:
              return null
          }
        })}
      </div>
    </article>
  )
}
