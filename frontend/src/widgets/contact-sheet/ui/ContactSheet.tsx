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

  return (
    <section aria-labelledby="contact-sheet-heading" className="mt-12">
      <Typography variant="title" id="contact-sheet-heading">
        {t('observation.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary mt-2">
        {t('observation.description')}
      </Typography>

      {/* The phone shape is a plain vertical list — a 240px card inside 328px of content left a
          27% dead gutter and squeezed the observation prose to 216px. The horizontal snap strip
          is the wide-screen shape only, and `sm:items-start` stops one verbose photo from making
          every card as tall as the tallest. */}
      <div className="mt-4 flex flex-col gap-4 sm:snap-x sm:flex-row sm:items-start sm:overflow-x-auto sm:overscroll-x-contain sm:pb-3">
        {images.map((image) => {
          const observation = observationsByFile.get(image.filename)
          // A just-confirmed upload may temporarily keep its browser blob preview in
          // the post cache. The contact sheet is a server-read surface: only the
          // presigned GetPost capability may fetch an R2 thumbnail.
          const viewUrl = image.viewUrl.startsWith('blob:') ? '' : image.viewUrl
          return (
            <article
              key={image.filename}
              className="bg-surface-raised w-full rounded-lg p-3 sm:w-60 sm:shrink-0 sm:snap-start"
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
