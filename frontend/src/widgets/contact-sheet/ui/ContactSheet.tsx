import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import { observationByFile } from '@/entities/observation'
import type { Observation } from '@/shared/api'

interface ContactSheetProps {
  images: readonly PostImage[]
  observations: readonly Observation[]
  activeJob?: GenerationJob
}

/** Persisted model eyesight, paired to photos only by their exact filenames. */
export function ContactSheet({ images, observations, activeJob }: ContactSheetProps) {
  const observationsByFile = observationByFile(observations)
  const observing =
    activeJob?.stage === 'observe' && activeJob.status !== 'done' && activeJob.status !== 'failed'

  return (
    <section aria-labelledby="contact-sheet-heading" className="mt-12">
      <h2 id="contact-sheet-heading" className="text-lg font-semibold tracking-tight">
        사진 관찰
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        모델이 각 사진에서 본 것 — 글이 이상하면 여기부터 확인하세요
      </p>

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
                  alt={`${image.filename} 관찰 사진`}
                  width={image.width}
                  height={image.height}
                  loading="lazy"
                  decoding="async"
                  className="aspect-square w-full rounded-md object-cover"
                />
              ) : (
                <div className="bg-surface-recessed text-content-tertiary flex aspect-square w-full items-center justify-center rounded-md px-3 text-center text-xs">
                  사진 주소를 준비하는 중…
                </div>
              )}
              <h3 className="mt-3 truncate text-sm font-medium">{image.filename}</h3>
              {observation ? (
                <dl className="mt-3 space-y-2">
                  <ObservationField label="장면" value={observation.scene} />
                  <ObservationField label="분위기" value={observation.mood} />
                  <ObservationField label="보이는 글자" value={observation.visibleText} />
                  <ObservationField label="사물" value={observation.objects.join(', ')} />
                </dl>
              ) : (
                // No per-card live region: ten waiting photos meant ten regions all announcing
                // '관찰 대기' over the one message that carries the count (the ProgressLine).
                <p className="text-content-tertiary mt-3 text-xs">
                  {observing ? '관찰 대기' : '관찰 결과 없음'}
                </p>
              )}
            </article>
          )
        })}
      </div>
    </section>
  )
}

function ObservationField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-content-tertiary text-xs">{label}</dt>
      {/* The body role, not the meta one: the section's own copy tells the user to read this, and
          the value is a model-supplied string, so it also breaks rather than overflowing (§3.2). */}
      <dd className="text-content-secondary mt-0.5 text-sm leading-relaxed break-words">
        {value || '없음'}
      </dd>
    </div>
  )
}
