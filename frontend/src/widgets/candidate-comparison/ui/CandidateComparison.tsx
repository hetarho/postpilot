import { useMemo } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import type {
  CandidateStatusName,
  ExperimentCandidate,
  ModelExperiment,
} from '@/entities/model-experiment'
import { AppFailureMessage, Badge, Notice, Typography, type BadgeTone } from '@/shared/ui'
import { formatNumber } from '@/shared/lib'
import { candidateSides, type CandidateSide } from '../model/sides'

interface CandidateComparisonProps {
  experiment: ModelExperiment
  activeCandidateId: string
}

const STATUS_TONES: Record<CandidateStatusName, BadgeTone> = {
  pending: 'neutral',
  running: 'neutral',
  succeeded: 'success',
  failed: 'danger',
}

/** The panels only. The A/B switch is docked by the page instead: it is pressed on every pass of
 *  the comparison, so it belongs in the same thumb band as the buttons that commit it, not pinned
 *  to the top edge ~700px away from the resting thumb (design-language §4.3). */
export function CandidateComparison({ experiment, activeCandidateId }: CandidateComparisonProps) {
  const { t } = useTranslation('posts')
  const sides = useMemo(() => candidateSides(experiment.candidates), [experiment.candidates])
  return (
    <div>
      <div className="grid gap-4 md:grid-cols-2">
        {sides.map(({ candidate, label }) => (
          <article
            key={candidate.id}
            aria-label={t('comparison.candidate', { label })}
            // The panel is NOT its own scroll container. A phone screen has one scroller, the
            // document (§4.4), and the inner one lost the reader's place on every switch because
            // `hidden` resets its scrollTop — paragraph-by-paragraph comparison, the whole point of
            // the screen, was impossible. The card is a `md:` treatment for the same reason: below
            // that only one panel is on screen, so a raised box around it frames the entire page
            // and costs two Korean characters per line of gutter (§1.4).
            className={`${candidate.id === activeCandidateId ? 'block' : 'hidden'} md:bg-surface-raised md:block md:rounded-lg md:p-4`}
          >
            <div className="flex min-h-11 items-center justify-between gap-2">
              <Typography variant="title">{t('comparison.candidate', { label })}</Typography>
              <Badge tone={STATUS_TONES[candidate.status]}>
                {t(`comparison.status.${candidate.status}`)}
              </Badge>
            </div>
            <CandidateOutput candidate={candidate} />
          </article>
        ))}
      </div>
      {experiment.revealed && <RevealBand sides={sides} />}
    </div>
  )
}

function CandidateOutput({ candidate }: { candidate: ExperimentCandidate }) {
  const { t } = useTranslation('posts')
  if (candidate.status === 'failed')
    return (
      <Notice tone="danger" role="alert" className="mt-4">
        {candidate.failure ? (
          <AppFailureMessage failure={candidate.failure} />
        ) : (
          t('comparison.failed')
        )}
      </Notice>
    )
  if (!candidate.output)
    return (
      <Typography variant="body" className="text-content-tertiary mt-4">
        {t('comparison.waiting')}
      </Typography>
    )
  if (candidate.output.kind === 'write') {
    const content = candidate.output.content
    return (
      <div className="mt-4">
        <Typography variant="title" as="h3">
          {content.title}
        </Typography>
        {content.summary && (
          <Typography variant="label" as="p" className="mt-2">
            {content.summary}
          </Typography>
        )}
        <div className="mt-5 space-y-4">
          {content.blocks.map((block, index) =>
            block.type === 5 ? (
              <Typography key={index} variant="body" as="ul" className="list-disc space-y-1 pl-5">
                {block.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </Typography>
            ) : block.type === 2 ? (
              <Typography key={index} variant="title" as="h4">
                {block.content}
              </Typography>
            ) : block.type === 3 ? (
              // `break-words`: the filename comes from the server (§3.2).
              <Typography
                key={index}
                variant="body"
                className="bg-surface-recessed rounded-md px-3 py-2 break-words"
              >
                {t('comparison.photo', { filename: block.file })}
                {block.caption ? ` — ${block.caption}` : ''}
              </Typography>
            ) : (
              <Typography key={index} variant="body" className="whitespace-pre-wrap">
                {block.content}
              </Typography>
            ),
          )}
        </div>
      </div>
    )
  }
  if (candidate.output.kind === 'observe') {
    return (
      <dl className="divide-divider mt-4 divide-y">
        {candidate.output.observations.map((item) => (
          <div key={item.file} className="py-4">
            <Typography variant="label" as="dt" className="text-content-primary break-words">
              {item.file}
            </Typography>
            <Typography variant="label" as="dd" className="mt-2 space-y-1">
              <p>
                <span className="text-content-tertiary">{t('comparison.scene')}</span>{' '}
                {item.scene || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">{t('comparison.mood')}</span>{' '}
                {item.mood || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">{t('comparison.visibleText')}</span>{' '}
                {item.visibleText || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">{t('comparison.objects')}</span>{' '}
                {item.objects.join(', ') || '—'}
              </p>
              <p>
                <span className="text-content-tertiary">{t('comparison.people')}</span>{' '}
                {item.peoplePresent ? t('comparison.present') : t('comparison.absent')}
              </p>
            </Typography>
          </div>
        ))}
      </dl>
    )
  }
  return (
    <Typography variant="body" as="div" className="mt-4 whitespace-pre-wrap">
      {candidate.output.styleguide}
    </Typography>
  )
}

/** Once the blind is lifted, both models' identity and accounting sit in ONE band below the
 *  panels. Inside the panels they could only ever be read one at a time below `md:`, at the very
 *  bottom of a post-length column — so comparing the two costs, the payoff of the whole exercise,
 *  meant memorising one number and switching (design-language §4.3). */
function RevealBand({ sides }: { sides: CandidateSide[] }) {
  const { t } = useTranslation('posts')
  return (
    <dl className="bg-surface-recessed divide-divider mt-6 divide-y rounded-lg px-4">
      {sides.map(({ candidate, label }) => (
        <div key={candidate.id} className="py-4">
          <dt className="flex flex-wrap items-baseline gap-x-2">
            <Typography variant="label" className="text-content-tertiary">
              {t('comparison.candidate', { label })}
            </Typography>
            <Typography variant="label" className="text-content-primary min-w-0">
              {candidate.modelLabel || t('comparison.modelUnavailable')}
            </Typography>
          </dt>
          <dd className="mt-1">
            {/* The label role, not the metadata one: after the reveal this is the most important
                content on the screen (§3). */}
            <Typography variant="label" as="p">
              {usageLine(candidate, t)}
            </Typography>
            {candidate.model && (
              <Typography variant="meta" as="p" mono className="mt-1 break-words">
                {candidate.model.providerId}/{candidate.model.modelId}
              </Typography>
            )}
          </dd>
        </div>
      ))}
    </dl>
  )
}

function usageLine(candidate: ExperimentCandidate, t: TFunction<'posts'>): string {
  const usage = candidate.usage
  if (!usage) return t('comparison.usageUnavailable')
  const cost =
    usage.costSource === 'unavailable'
      ? t('comparison.costUnavailable')
      : `${usage.costSource === 'estimated' ? '≈ ' : ''}${formatNumber(
          Number(usage.costMicrousd) / 1_000_000,
          undefined,
          {
            style: 'currency',
            currency: 'USD',
            currencyDisplay: 'narrowSymbol',
            minimumFractionDigits: 6,
            maximumFractionDigits: 6,
          },
        )}`
  return t('comparison.usage', {
    prompt: formatNumber(usage.promptTokens),
    completion: formatNumber(usage.completionTokens),
    latency: formatNumber(usage.latencyMs),
    cost,
  })
}
