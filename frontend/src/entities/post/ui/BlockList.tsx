import { Fragment, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { BlockType, type PostContent } from '@/shared/api'
import type { PostImage } from '@/entities/image/@x/post'
import { blockKey, imageByFile } from '../model/content'

interface BlockListProps {
  content: PostContent
  images: readonly PostImage[]
  /** Wraps one rendered block, so a consumer can put its own affordance around the reading view
   *  without this entity gaining one. The entities layer may not depend on features, so the edit
   *  control has to arrive from the outside — the same render-prop seam `BlockEditor` already uses
   *  for sentence feedback. */
  renderBlock?: (
    block: PostContent['blocks'][number],
    index: number,
    rendered: ReactNode,
  ) => ReactNode
  /** Wraps the title/summary/tags header for the same reason. */
  renderHeader?: (rendered: ReactNode) => ReactNode
}

/** The canonical block array rendered as a read-only draft. */
export function BlockList({ content, images, renderBlock, renderHeader }: BlockListProps) {
  const { t } = useTranslation('posts')
  const imagesByFile = imageByFile(images)
  // The key stays on this wrapper rather than on whatever the consumer returns — and it is the
  // block's POSITION, not `blockKey`'s content-derived string: a consumer that edits the block in
  // place would otherwise remount it on every keystroke and lose both the open editor and the caret.
  const wrap = (block: PostContent['blocks'][number], index: number, rendered: ReactNode) =>
    renderBlock ? <Fragment key={index}>{renderBlock(block, index, rendered)}</Fragment> : rendered

  return (
    <article aria-label={t('generatedContent')} className="mt-12 pb-12">
      {(renderHeader ?? ((rendered: ReactNode) => rendered))(
        <header>
          <p className="text-content-tertiary text-xs font-medium tracking-wide uppercase">
            {t('draftLabel')}
          </p>
          {/* `break-words` on every model-supplied string in this article: the global rule keeps the
            page from scrolling sideways, the local class keeps the box from being the one that
            overflows (design-language §3.2). A model routinely writes a bare URL or a model id. */}
          <h1 className="mt-1 text-2xl font-semibold tracking-tight break-words">
            {content.title}
          </h1>
          {content.summary && (
            <p className="text-content-secondary mt-3 text-sm leading-relaxed break-words">
              {content.summary}
            </p>
          )}
          {content.tags.length > 0 && (
            <ul aria-label={t('tags')} className="mt-3 flex flex-wrap gap-2">
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
        </header>,
      )}

      <div className="mt-8 space-y-5">
        {content.blocks.map((block, index) => {
          const key = blockKey(block, index)
          switch (block.type) {
            case BlockType.TEXT:
              return wrap(
                block,
                index,
                <p key={key} className="text-sm leading-relaxed break-words whitespace-pre-wrap">
                  {block.content}
                </p>,
              )
            case BlockType.HEADING:
              return wrap(
                block,
                index,
                block.level === 3 ? (
                  <h3 key={key} className="pt-3 text-base font-semibold tracking-tight">
                    {block.content}
                  </h3>
                ) : (
                  <h2 key={key} className="pt-5 text-lg font-semibold tracking-tight">
                    {block.content}
                  </h2>
                ),
              )
            case BlockType.IMAGE: {
              const image = imagesByFile.get(block.file)
              if (!image) return null
              return wrap(
                block,
                index,
                <div key={key} className="py-2">
                  {/* Not the photo strip's `Thumbnail`: that tile is a fixed 128px square with
                      `object-cover`, so in the finished draft the photo the post exists for
                      rendered at a third of the column with the top and bottom of a portrait shot
                      cropped away (design-language §0). Here it fills the column at its own
                      aspect ratio, which `width`/`height` reserve before the pixels land (§8.6). */}
                  {image.viewUrl ? (
                    <img
                      src={image.viewUrl}
                      alt={block.alt || block.file}
                      width={image.width}
                      height={image.height}
                      loading="lazy"
                      decoding="async"
                      className="bg-surface-recessed h-auto w-full rounded-lg"
                    />
                  ) : (
                    // The view URL is minted per GetPost; until one arrives the box is still held
                    // open so the text below it does not jump when the photo paints.
                    <div className="bg-surface-recessed aspect-square w-full rounded-lg" />
                  )}
                  {block.caption && (
                    <p className="text-content-tertiary mt-2 text-xs break-words">
                      {block.caption}
                    </p>
                  )}
                </div>,
              )
            }
            case BlockType.QUOTE:
              return wrap(
                block,
                index,
                <blockquote
                  key={key}
                  className="bg-surface-recessed text-content-secondary rounded-md px-4 py-3 text-sm leading-relaxed break-words"
                >
                  {block.content}
                </blockquote>,
              )
            case BlockType.LIST:
              return wrap(
                block,
                index,
                <ul
                  key={key}
                  className="list-disc space-y-1 pl-5 text-sm leading-relaxed break-words"
                >
                  {block.items.map((item, itemIndex) => (
                    <li key={`${item}:${itemIndex}`}>{item}</li>
                  ))}
                </ul>,
              )
            default:
              return null
          }
        })}
      </div>
    </article>
  )
}
