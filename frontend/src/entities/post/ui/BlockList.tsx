import { Fragment, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { twMerge } from 'tailwind-merge'
import { BlockType, type PostContent } from '@/shared/api'
import { Typography } from '@/shared/ui'
import type { PostImage } from '@/entities/image/@x/post'
import { blockKey, imageByFile } from '../model/content'

interface BlockListProps {
  content: PostContent
  images: readonly PostImage[]
  /** The article's accessible name. The editor's reading view keeps the default; a second
   *  rendering on the same page (the Naver export preview) names itself apart so the two articles
   *  stay distinguishable to a screen reader. */
  label?: string
  className?: string
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
  /** Rendered in place of an IMAGE block whose file has no matching image. The default (nothing)
   *  is right for the reading and editing views; the export preview holds the marker's position
   *  with its own placeholder, because a dropped position would shift every later photo against
   *  its `[사진 …]` marker. */
  renderMissingImage?: (block: PostContent['blocks'][number], index: number) => ReactNode
}

/** The canonical block array rendered as a read-only draft. */
export function BlockList({
  content,
  images,
  label,
  className,
  renderBlock,
  renderHeader,
  renderMissingImage,
}: BlockListProps) {
  const { t } = useTranslation('posts')
  const imagesByFile = imageByFile(images)
  // The key stays on this wrapper rather than on whatever the consumer returns — and it is the
  // block's POSITION, not `blockKey`'s content-derived string: a consumer that edits the block in
  // place would otherwise remount it on every keystroke and lose both the open editor and the caret.
  const wrap = (block: PostContent['blocks'][number], index: number, rendered: ReactNode) =>
    renderBlock ? <Fragment key={index}>{renderBlock(block, index, rendered)}</Fragment> : rendered

  return (
    <article
      aria-label={label ?? t('generatedContent')}
      className={twMerge('mt-12 pb-12', className)}
    >
      {(renderHeader ?? ((rendered: ReactNode) => rendered))(
        <header>
          <Typography variant="eyebrow" as="p">
            {t('draftLabel')}
          </Typography>
          {/* `break-words` on every model-supplied string in this article: the global rule keeps the
            page from scrolling sideways, the local class keeps the box from being the one that
            overflows (design-language §3.2). A model routinely writes a bare URL or a model id. */}
          {/* The bare editor title is the screen's one `display`. This generated article title is
              a nested reading section, so it keeps heading semantics without competing with the
              page title for the top visual role (design-language §3 / Job 38 A10). */}
          <Typography variant="title" as="h3" className="mt-1 break-words">
            {content.title}
          </Typography>
          {content.summary && (
            <Typography variant="body" className="text-content-secondary mt-3 break-words">
              {content.summary}
            </Typography>
          )}
          {content.tags.length > 0 && (
            <ul aria-label={t('tags')} className="mt-3 flex flex-wrap gap-2">
              {content.tags.map((tag) => (
                <Typography
                  variant="meta"
                  as="li"
                  key={tag}
                  className="bg-surface-raised text-content-secondary rounded-sm px-2 py-1 break-words"
                >
                  #{tag}
                </Typography>
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
              // An unfilled template slot rides on a TEXT block (spec/tech/post-template-grammar
              // §7). It renders as the position it reserves rather than as its own content —
              // the content is the copy token, which is machinery and not prose.
              if (block.slot)
                return wrap(block, index, <SlotRow key={key} label={block.slot.label} />)
              return wrap(
                block,
                index,
                <Typography variant="body" key={key} className="break-words whitespace-pre-wrap">
                  {block.content}
                </Typography>,
              )
            case BlockType.HEADING:
              return wrap(
                block,
                index,
                block.level === 3 ? (
                  <Typography variant="title" as="h5" key={key} className="pt-3 break-words">
                    {block.content}
                  </Typography>
                ) : (
                  <Typography variant="title" as="h4" key={key} className="pt-5 break-words">
                    {block.content}
                  </Typography>
                ),
              )
            case BlockType.IMAGE: {
              const image = imagesByFile.get(block.file)
              if (!image)
                return renderMissingImage ? (
                  <Fragment key={key}>{renderMissingImage(block, index)}</Fragment>
                ) : null
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
                    <Typography variant="label" as="p" className="mt-2 break-words">
                      {block.caption}
                    </Typography>
                  )}
                </div>,
              )
            }
            case BlockType.QUOTE:
              return wrap(
                block,
                index,
                <Typography
                  variant="body"
                  as="blockquote"
                  key={key}
                  className="bg-surface-recessed text-content-secondary rounded-md px-4 py-3 break-words"
                >
                  {block.content}
                </Typography>,
              )
            case BlockType.LIST:
              return wrap(
                block,
                index,
                <Typography
                  variant="body"
                  as="ul"
                  key={key}
                  className="list-disc space-y-1 pl-5 break-words"
                >
                  {block.items.map((item, itemIndex) => (
                    <li key={`${item}:${itemIndex}`}>{item}</li>
                  ))}
                </Typography>,
              )
            default:
              return null
          }
        })}
      </div>
    </article>
  )
}

/** One position a template reserved and nobody has filled yet. A stepped surface rather than a
 *  border or a card: it is one line inside a flowing article, and §1.3/§1.4 both point away from
 *  drawing a box around it. The label is the template author's own words. */
function SlotRow({ label }: { label: string }) {
  const { t } = useTranslation(['templates', 'posts'])
  return (
    <div className="bg-surface-recessed rounded-md px-3 py-2">
      <Typography variant="eyebrow" as="p">
        {t('slot.unfilled', { ns: 'templates' })}
      </Typography>
      {label && (
        <Typography variant="body" className="text-content-secondary mt-0.5 break-words">
          {label}
        </Typography>
      )}
    </div>
  )
}
