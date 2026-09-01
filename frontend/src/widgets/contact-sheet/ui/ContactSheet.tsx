import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import { observationByFile } from '@/entities/observation'
import type { Observation } from '@/shared/api'
import { Typography } from '@/shared/ui'

interface ContactSheetProps {
  images: readonly PostImage[]
  observations: readonly Observation[]
  activeJob?: GenerationJob
}

/** Persisted model eyesight, paired to photos only by their exact filenames. */
export function ContactSheet({ images, observations, activeJob }: ContactSheetProps) {
  const { t } = useTranslation('posts')
  const observationsByFile = observationByFile(observations)
  const observing =
    activeJob?.stage === 'observe' && activeJob.status !== 'done' && activeJob.status !== 'failed'
  const [current, setCurrent] = useState(0)
  const stripRef = useRef<HTMLDivElement>(null)

  // An IntersectionObserver, not a scroll handler: reading a card's offset on every scroll frame
  // forces a layout on the thread that is trying to animate the swipe. The observer reports the
  // card that is mostly on screen, which is the one the snap has settled on.
  useEffect(() => {
    const strip = stripRef.current
    if (!strip || typeof IntersectionObserver === 'undefined') return
    const cards = Array.from(strip.children)
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
        if (!visible) return
        const index = cards.indexOf(visible.target)
        if (index >= 0) setCurrent(index)
      },
      { root: strip, threshold: 0.6 },
    )
    for (const card of cards) observer.observe(card)
    return () => observer.disconnect()
  }, [images])

  return (
    <section aria-labelledby="contact-sheet-heading" className="mt-12">
      <Typography variant="title" id="contact-sheet-heading">
        {t('observation.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary mt-2">
        {t('observation.description')}
      </Typography>

      {/* One horizontal snap carousel on a phone: eight photos as full-width cards was eight
          screenfuls of vertical scrolling to reach the last observation. A horizontal strip is
          §4.4's deliberate exception — it does not compete with the page's vertical scroll — and
          `overscroll-x-contain` keeps a swipe that reaches the end off the browser's back gesture.
          The card is deliberately narrower than the strip so a SLIVER of the next one shows: a
          phone has no hover and no scrollbar, so the sliver is the only thing that says the strip
          scrolls at all. `items-start` stops one verbose photo from making every card as tall as
          the tallest, and no card gains an inner vertical scroller — a long observation makes its
          own card taller. */}
      <div
        ref={stripRef}
        className="mt-4 flex snap-x snap-mandatory items-start gap-4 overflow-x-auto overscroll-x-contain pb-3"
      >
        {images.map((image) => {
          const observation = observationsByFile.get(image.filename)
          // A just-confirmed upload may temporarily keep its browser blob preview in
          // the post cache. The contact sheet is a server-read surface: only the
          // presigned GetPost capability may fetch an R2 thumbnail.
          const viewUrl = image.viewUrl.startsWith('blob:') ? '' : image.viewUrl
          return (
            <article
              key={image.filename}
              className="bg-surface-raised w-carousel-card shrink-0 snap-start rounded-lg p-3 sm:w-60"
            >
              {viewUrl ? (
                <img
                  src={viewUrl}
                  alt={t('observation.imageAlt', { filename: image.filename })}
                  width={image.width}
                  height={image.height}
                  loading="lazy"
                  decoding="async"
                  className="aspect-square w-full rounded-md object-cover"
                />
              ) : (
                <Typography
                  variant="body"
                  as="div"
                  className="bg-surface-recessed flex aspect-square w-full items-center justify-center rounded-md px-3 text-center"
                >
                  {t('observation.urlPending')}
                </Typography>
              )}
              <Typography variant="label" as="h3" className="text-content-primary mt-3 truncate">
                {image.filename}
              </Typography>
              {observation ? (
                <dl className="mt-3 space-y-2">
                  <ObservationField label={t('observation.scene')} value={observation.scene} />
                  <ObservationField label={t('observation.mood')} value={observation.mood} />
                  <ObservationField
                    label={t('observation.visibleText')}
                    value={observation.visibleText}
                  />
                  <ObservationField
                    label={t('observation.objects')}
                    value={observation.objects.join(', ')}
                  />
                </dl>
              ) : (
                // No per-card live region: ten waiting photos meant ten regions all announcing
                // '관찰 대기' over the one message that carries the count (the ProgressLine).
                <Typography variant="body" as="p" className="text-content-tertiary mt-3">
                  {observing ? t('observation.waiting') : t('observation.empty')}
                </Typography>
              )}
            </article>
          )
        })}
      </div>
      {/* Where you are in the strip, since only one card and a sliver are on screen. Hidden from
          `sm:` up, where several cards are visible at once and the count says nothing new. */}
      {images.length > 1 && (
        <Typography
          variant="meta"
          as="p"
          role="status"
          className="text-content-tertiary mt-2 sm:hidden"
        >
          {t('observation.position', { current: current + 1, total: images.length })}
        </Typography>
      )}
    </section>
  )
}

function ObservationField({ label, value }: { label: string; value: string }) {
  const { t } = useTranslation('posts')
  return (
    <div>
      <Typography variant="label" as="dt">
        {label}
      </Typography>
      {/* The body role, not the meta one: the section's own copy tells the user to read this, and
          the value is a model-supplied string, so it also breaks rather than overflowing (§3.2). */}
      <Typography variant="body" as="dd" className="text-content-secondary mt-0.5 break-words">
        {value || t('observation.none')}
      </Typography>
    </div>
  )
}
