import { useMemo, type ReactNode } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import type {
  CandidateSide,
  CandidateStatusName,
  ExperimentCandidate,
  ModelExperiment,
} from '@/entities/model-experiment'
import { candidateSides } from '@/entities/model-experiment'
import { badgeGroup, badgeTone } from '@/entities/model-experiment'
import { AppFailureMessage, Badge, Notice, Typography, type BadgeTone } from '@/shared/ui'
import type { FingerprintComparisonItem } from '@/entities/voice'
import { formatNumber } from '@/shared/lib'
import { BlockType, GalleryLayout } from '@/shared/api'

interface CandidateComparisonProps {
  experiment: ModelExperiment
  activeCandidateId: string
  /** Draws a 말투 반영 비교 piece's fingerprint comparison under the piece (MODEL-67); the page
   *  supplies it, since this widget does not own the comparison's wording. */
  renderComparison?: (items: FingerprintComparisonItem[]) => ReactNode
}

const STATUS_TONES: Record<CandidateStatusName, BadgeTone> = {
  pending: 'neutral',
  running: 'neutral',
  succeeded: 'success',
  failed: 'danger',
}

/** The panels only. The A/B switch is docked by the page instead: it is pressed on every pass of
 *  the comparison, so it belongs in the same thumb band as the buttons that commit it, not pinned
 *  to the top edge ~700px away from the resting thumb (THEME-24). */
export function CandidateComparison({
  experiment,
  activeCandidateId,
  renderComparison,
}: CandidateComparisonProps) {
  const { t } = useTranslation('posts')
  const sides = useMemo(() => candidateSides(experiment.candidates), [experiment.candidates])
  return (
    <div>
      {/* The owner's own answer on top: both pieces are read against it (MODEL-67). It leaves with
          the snapshot once the comparison's content is purged. */}
      {experiment.source === 'voice' && (
        <section
          aria-label={t('comparison.voiceAnswer')}
          className="bg-surface-recessed mb-4 rounded-lg px-4 py-3"
        >
          <Typography variant="label" as="h3">
            {experiment.voicePromptText || t('comparison.voiceAnswer')}
          </Typography>
          <Typography variant="body" as="p" className="mt-2 break-words whitespace-pre-line">
            {experiment.voiceAnswer ? (
              <>
                <span className="text-content-secondary">{t('comparison.voiceAnswer')}</span>{' '}
                {experiment.voiceAnswer}
              </>
            ) : (
              <span className="text-content-secondary">{t('comparison.voiceAnswerPurged')}</span>
            )}
          </Typography>
        </section>
      )}
      <div className="grid gap-4 md:grid-cols-2">
        {sides.map(({ candidate, label }) => (
          <article
            key={candidate.id}
            aria-label={t('comparison.candidate', { label })}
            // The panel is NOT its own scroll container. A phone screen has one scroller, the
            // document (THEME-25), and the inner one lost the reader's place on every switch because
            // `hidden` resets its scrollTop — paragraph-by-paragraph comparison, the whole point of
            // the screen, was impossible. The card is a `md:` treatment for the same reason: below
            // that only one panel is on screen, so a raised box around it frames the entire page
            // and costs two Korean characters per line of gutter (THEME-13).
            className={`${candidate.id === activeCandidateId ? 'block' : 'hidden'} md:bg-surface-raised min-w-0 break-words md:block md:rounded-lg md:p-4`}
          >
            <div className="flex min-h-11 items-center justify-between gap-2">
              <Typography variant="title">{t('comparison.candidate', { label })}</Typography>
              <Badge tone={STATUS_TONES[candidate.status]}>
                {t(`comparison.status.${candidate.status}`)}
              </Badge>
            </div>
            <CandidateOutput candidate={candidate} renderComparison={renderComparison} />
          </article>
        ))}
      </div>
      {experiment.revealed &&
        experiment.reviewMode === 'candidate_ranking' &&
        !sides.some(({ candidate }) => candidate.rank) && (
          <Typography variant="label" as="p" className="mt-6">
            {t('comparison.noRankingRecorded')}
          </Typography>
        )}
      {experiment.revealed && (
        <RevealBand sides={sides} ranked={experiment.reviewMode === 'candidate_ranking'} />
      )}
    </div>
  )
}

function CandidateOutput({
  candidate,
  renderComparison,
}: {
  candidate: ExperimentCandidate
  renderComparison?: (items: FingerprintComparisonItem[]) => ReactNode
}) {
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
        {t(candidate.status === 'succeeded' ? 'comparison.expired' : 'comparison.waiting')}
      </Typography>
    )
  if (candidate.output.kind === 'voice') {
    return (
      <div className="mt-4">
        <Typography variant="body" as="p" className="break-words whitespace-pre-line">
          {candidate.output.text}
        </Typography>
        {renderComparison && (
          <div className="mt-4">{renderComparison(candidate.output.comparison)}</div>
        )}
      </div>
    )
  }
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
        {content.tags.length > 0 && (
          <ul aria-label={t('tags')} className="mt-3 flex flex-wrap gap-2">
            {content.tags.map((tag, index) => (
              <Typography variant="label" as="li" key={`${tag}:${index}`} className="break-words">
                #{tag}
              </Typography>
            ))}
          </ul>
        )}
        <div className="mt-5 space-y-4">
          {content.blocks.map((block, index) =>
            block.type === BlockType.LIST ? (
              <Typography key={index} variant="body" as="ul" className="list-disc space-y-1 pl-5">
                {block.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </Typography>
            ) : block.type === BlockType.HEADING ? (
              <Typography key={index} variant="title" as="h4">
                {block.content}
              </Typography>
            ) : block.type === BlockType.IMAGE ? (
              // `break-words`: the filename comes from the server (THEME-21).
              <Typography
                key={index}
                variant="body"
                className="bg-surface-recessed rounded-md px-3 py-2 break-words"
              >
                {t('comparison.photo', { filename: block.file })}
                {block.caption ? `: ${block.caption}` : ''}
              </Typography>
            ) : block.type === BlockType.VIDEO ? (
              <Typography
                key={index}
                variant="body"
                className="bg-surface-recessed rounded-md px-3 py-2 break-words"
              >
                {t('comparison.video', { filename: block.file })}
                {block.caption ? `: ${block.caption}` : ''}
              </Typography>
            ) : block.type === BlockType.QUOTE ? (
              <Typography
                key={index}
                variant="body"
                as="blockquote"
                className="break-words whitespace-pre-wrap"
              >
                {block.content}
              </Typography>
            ) : block.type === BlockType.GALLERY ? (
              // A photo group is one line, as it is one place in the post (GEN-77); layout 2 is
              // SLIDE, anything else reads as a collage (GEN-78).
              <Typography
                key={index}
                variant="body"
                className="bg-surface-recessed rounded-md px-3 py-2 break-words"
              >
                {t('comparison.photoGroup', {
                  layout: t(
                    block.layout === GalleryLayout.SLIDE
                      ? 'comparison.layout.slide'
                      : 'comparison.layout.collage',
                  ),
                  filenames: block.files.join(', '),
                })}
                {block.caption ? `: ${block.caption}` : ''}
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
              {item.scene || '-'}
            </p>
            <p>
              <span className="text-content-tertiary">{t('comparison.mood')}</span>{' '}
              {item.mood || '-'}
            </p>
            <p>
              <span className="text-content-tertiary">{t('comparison.visibleText')}</span>{' '}
              {item.visibleText || '-'}
            </p>
            <p>
              <span className="text-content-tertiary">{t('comparison.objects')}</span>{' '}
              {item.objects.join(', ') || '-'}
            </p>
            <p>
              <span className="text-content-tertiary">{t('comparison.people')}</span>{' '}
              {item.peoplePresent ? t('comparison.present') : t('comparison.absent')}
            </p>
            {item.events.length > 0 && (
              <div>
                <Typography variant="label" as="p" className="text-content-tertiary">
                  {t('comparison.events')}
                </Typography>
                <Typography variant="label" as="ol" className="list-decimal space-y-1 pl-5">
                  {item.events.map((event, index) => (
                    <li key={`${index}:${event}`} className="break-words whitespace-pre-wrap">
                      {event}
                    </li>
                  ))}
                </Typography>
              </div>
            )}
            {item.speech && (
              <p className="break-words whitespace-pre-wrap">
                <span className="text-content-tertiary">{t('comparison.speech')}</span>{' '}
                {item.speech}
              </p>
            )}
          </Typography>
        </div>
      ))}
    </dl>
  )
}

/** Once the blind is lifted, both models' identity and accounting sit in ONE band below the
 *  panels. Inside the panels they could only ever be read one at a time below `md:`, at the very
 *  bottom of a post-length column — so comparing usage and model identity meant memorising
 *  one result and switching (THEME-24). */
function RevealBand({ sides, ranked }: { sides: CandidateSide[]; ranked: boolean }) {
  const { t } = useTranslation(['posts', 'models'])
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
            {ranked && candidate.rank && (
              <Typography variant="label" as="p" className="text-content-primary">
                {t('comparison.historicalRank', { rank: candidate.rank })}
                {sides.filter(({ candidate: other }) => other.rank === candidate.rank).length > 1
                  ? ` · ${t('comparison.historicalTie')}`
                  : ''}
              </Typography>
            )}
            {/* The label role, not the metadata one: after the reveal this is the most important
                content on the screen (THEME-19). */}
            <Typography variant="label" as="p">
              {usageLine(candidate, t)}
            </Typography>
            {candidate.model && (
              <Typography variant="meta" as="p" mono className="mt-1 break-words">
                {candidate.model.providerId}/{candidate.model.modelId}
              </Typography>
            )}
            {/* What the verdict said about this candidate, beside the identity it was said
                about. Read here rather than in the panels because the reason one result won
                is only legible next to the reason the other lost. */}
            {(candidate.badges.length > 0 || candidate.otherNote) && (
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {/* Positive, then negative, then `기타` below both in its own tone (MODEL-62). */}
                {(['positive', 'negative', 'other'] as const)
                  .flatMap((group) =>
                    candidate.badges.filter((badge) => badgeGroup(badge) === group),
                  )
                  .map((badge) => (
                    <Badge key={badge} tone={badgeTone(badge)}>
                      {t(`badge.${badge}`, { ns: 'models' })}
                    </Badge>
                  ))}
                {candidate.otherNote && (
                  <Typography variant="meta" as="p" className="w-full break-words">
                    {candidate.otherNote}
                  </Typography>
                )}
              </div>
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
  return t('comparison.usage', {
    prompt: formatNumber(usage.promptTokens),
    completion: formatNumber(usage.completionTokens),
    latency: formatNumber(usage.latencyMs),
  })
}
