import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { GalleryLayout, type Block } from '@/shared/api'
import { prefersReducedMotion } from '@/shared/lib'
import { Button, Typography } from '@/shared/ui'
import type { PostImage } from '@/entities/image/@x/post'

interface PhotoGroupProps {
  /** A GALLERY block: its files, layout, alt and the one caption that belongs to the group. */
  block: Block
  images: ReadonlyMap<string, PostImage>
  /** Renders one photo's cell in place of the plain photo. The export preview turns each photo
   *  into its own copy control (EXPORT-12). `fit` is how the layout shows a photo: cropped to an
   *  even square in a collage, whole inside a square in a slide. A consumer that renders its own
   *  cells owns their shape, so the group no longer clips them to the square. */
  renderPhoto?: (file: string, position: number, image: PostImage, fit: PhotoFit) => ReactNode
  /** Holds the cell of a photo the post no longer has. The default is a placeholder naming the
   *  file, so the group keeps its shape (POST-105). */
  renderMissingPhoto?: (file: string, position: number, fit: PhotoFit) => ReactNode
  /** Replaces the caption under the group; the export preview puts its caption copy there. */
  renderCaption?: (block: Block) => ReactNode
}

/** How a layout shows one photo: a collage crops it to an even square cell, a slide shows it whole
 *  inside a square. */
export type PhotoFit = 'cover' | 'contain'

/** A photo group as one place (POST-105): 콜라주 lays the photos side by side in rows, 슬라이드 shows
 *  one at a time in a snap strip, and either way the group's one caption stands under it. */
export function PhotoGroup({
  block,
  images,
  renderPhoto,
  renderMissingPhoto,
  renderCaption,
}: PhotoGroupProps) {
  const { t } = useTranslation('posts')
  const total = block.files.length
  const custom = Boolean(renderPhoto || renderMissingPhoto)
  const cell = (file: string, position: number, fit: PhotoFit) => {
    const image = images.get(file)
    if (!image) {
      return renderMissingPhoto ? (
        renderMissingPhoto(file, position, fit)
      ) : (
        <div className="bg-surface-recessed flex h-full w-full items-center justify-center rounded-lg p-2">
          <Typography variant="meta" as="p" className="text-content-tertiary break-all">
            {file}
          </Typography>
        </div>
      )
    }
    if (renderPhoto) return renderPhoto(file, position, image, fit)
    return image.viewUrl ? (
      <img
        src={image.viewUrl}
        // The group's one alt names the whole group; the position keeps two of its photos from
        // reading as the same image to a screen reader.
        alt={t('photoGroup.photoAlt', {
          alt: block.alt || file,
          current: position + 1,
          total,
        })}
        width={image.width}
        height={image.height}
        loading="lazy"
        decoding="async"
        className={
          fit === 'cover'
            ? 'bg-surface-recessed h-full w-full rounded-lg object-cover'
            : 'h-full w-full object-contain'
        }
      />
    ) : (
      // The view URL is minted per GetPost; until one arrives the cell is still held open.
      <div className="bg-surface-recessed h-full w-full rounded-lg" />
    )
  }
  const caption = renderCaption
    ? renderCaption(block)
    : block.caption && (
        <Typography variant="label" as="p" className="mt-2 break-words">
          {block.caption}
        </Typography>
      )

  return (
    <figure className="py-2">
      {block.layout === GalleryLayout.SLIDE ? (
        <SlideStrip files={block.files} cell={cell} custom={custom} />
      ) : (
        // One row, as many columns as photos: a group holds at most three, so its caption stands
        // right under every photo it describes. A collage crops each photo to an even square cell,
        // as Naver's does.
        <div className={`grid gap-2 ${total === 2 ? 'grid-cols-2' : 'grid-cols-3'}`}>
          {block.files.map((file, position) => (
            <div
              key={`${file}:${position}`}
              className={custom ? 'min-w-0' : 'aspect-square overflow-hidden rounded-lg'}
            >
              {cell(file, position, 'cover')}
            </div>
          ))}
        </div>
      )}
      {caption}
    </figure>
  )
}

/** 슬라이드: one photo at a time in a horizontal snap strip (THEME-25's strip exception), with the
 *  position and previous/next buttons beside the swipe — a swipe alone is unreachable with a
 *  mouse (POST-105). */
function SlideStrip({
  files,
  cell,
  custom,
}: {
  files: readonly string[]
  cell: (file: string, position: number, fit: PhotoFit) => ReactNode
  custom: boolean
}) {
  const { t } = useTranslation('posts')
  const [current, setCurrent] = useState(0)
  const stripRef = useRef<HTMLDivElement>(null)

  // An IntersectionObserver rather than a scroll handler, as the contact sheet does: the slide
  // that is mostly on screen is the one the snap settled on, and nothing reads layout per frame.
  useEffect(() => {
    const strip = stripRef.current
    if (!strip || typeof IntersectionObserver === 'undefined') return
    const slides = Array.from(strip.children)
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
        if (!visible) return
        const index = slides.indexOf(visible.target)
        if (index >= 0) setCurrent(index)
      },
      { root: strip, threshold: 0.6 },
    )
    for (const slide of slides) observer.observe(slide)
    return () => observer.disconnect()
  }, [files])

  const go = (index: number) => {
    const strip = stripRef.current
    if (!strip) return
    // The observer moves the indicator once the snap settles; setting it here as well keeps the
    // buttons right where no observer runs.
    setCurrent(index)
    strip.scrollTo({
      left: index * strip.clientWidth,
      behavior: prefersReducedMotion() ? 'auto' : 'smooth',
    })
  }

  return (
    <div>
      <div
        ref={stripRef}
        className="flex snap-x snap-mandatory overflow-x-auto overscroll-x-contain rounded-lg"
      >
        {files.map((file, position) => (
          <div
            key={`${file}:${position}`}
            className={
              custom
                ? 'w-full shrink-0 snap-center'
                : 'bg-surface-recessed aspect-square w-full shrink-0 snap-center'
            }
          >
            {cell(file, position, 'contain')}
          </div>
        ))}
      </div>
      <div className="mt-2 flex items-center justify-between gap-2">
        <Button
          variant="ghost"
          aria-label={t('photoGroup.previous')}
          disabled={current === 0}
          onClick={() => go(current - 1)}
        >
          ‹
        </Button>
        <Typography variant="meta" as="p" role="status" className="text-content-tertiary">
          {t('photoGroup.position', { current: current + 1, total: files.length })}
        </Typography>
        <Button
          variant="ghost"
          aria-label={t('photoGroup.next')}
          disabled={current >= files.length - 1}
          onClick={() => go(current + 1)}
        >
          ›
        </Button>
      </div>
    </div>
  )
}
