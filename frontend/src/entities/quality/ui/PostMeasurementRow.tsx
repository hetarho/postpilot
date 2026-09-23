import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { ContentLanguage } from '@/shared/api'
import { Badge, Button, Typography, typographyStyles } from '@/shared/ui'
import { usePostMeasurement } from '../api/usePostMeasurement'
import type { i18n } from '../config/i18n'
import {
  absentValueLabel,
  bandsAreOwnLine,
  belowMinimumLine,
  formatMeasure,
  formatShare,
  qualityMetricName,
} from '../model/format'
import type { QualityMetricId, QualityReading, QualityValues, QualityVerdict } from '../model/types'

/** The three metrics a post has numbers of its own for. M1 is the account's alone and appears only
 *  in ①'s brief (QUAL-36), so a reading of it is never drawn here. */
type PostMetricId = Exclude<QualityMetricId, 'title_saturation'>

function isPostMetric(reading: QualityReading): reading is QualityReading & {
  metric: PostMetricId
} {
  return reading.metric !== 'title_saturation'
}

/** One value line: what is counted, its number or 측정할 수 없어요, and the band edge the server
 *  sent, worded in the direction that is fine — the client mirrors no number (QUAL-6). */
interface ValueLine {
  key: string
  name: string
  value: string
  band?: string
}

type ValueName = keyof (typeof i18n)['ko']['quality']['value']

function valuesOf<M extends QualityMetricId>(
  reading: QualityReading,
  metric: M,
): Extract<QualityValues, { metric: M }> | undefined {
  const values = reading.values
  return values?.metric === metric ? (values as Extract<QualityValues, { metric: M }>) : undefined
}

export function PostMeasurementRow({
  ownerId,
  slug,
  revision,
  contentLanguage,
  className,
}: {
  ownerId: string
  slug: string
  /** The content revision the row describes; a new one reads a new measurement (QUAL-3). */
  revision: bigint
  /** The language the content is written in, which decides whether a sentence is measured in
   *  characters or in words. A post with none reads as Korean (QUAL-26). */
  contentLanguage?: ContentLanguage
  className?: string
}) {
  const { t } = useTranslation(['posts', 'common'])
  const headingId = useId()
  const { measurement, isError, refetch, isFetching } = usePostMeasurement(ownerId, slug, revision)

  const share = (value: number | undefined) =>
    value === undefined ? absentValueLabel() : formatShare(value)
  const amount = (key: 'chars' | 'photos' | 'types', value: number | undefined) =>
    value === undefined
      ? absentValueLabel()
      : t(`quality.amount.${key}`, { ns: 'posts', value: formatMeasure(value) })
  const sentence = (value: number | undefined) =>
    value === undefined
      ? absentValueLabel()
      : t(
          contentLanguage === 'en'
            ? 'quality.amount.sentenceWords'
            : 'quality.amount.sentenceChars',
          { ns: 'posts', count: value, value: formatMeasure(value) },
        )
  const band = (direction: 'atMost' | 'atLeast' | 'above', edge: string) =>
    t(`quality.band.${direction}`, { ns: 'posts', edge })
  const name = (key: ValueName) => t(`quality.value.${key}`, { ns: 'posts' })

  const LINES: Record<PostMetricId, (reading: QualityReading) => ValueLine[]> = {
    cross_post_phrases: (reading) => {
      const values = valuesOf(reading, 'cross_post_phrases')
      return [
        {
          key: 'share',
          name: name('sharedWithPublished'),
          value: share(values?.share),
          band: values && band('atMost', formatShare(values.shareWarnAbove)),
        },
      ]
    },
    in_post_repetition: (reading) => {
      const values = valuesOf(reading, 'in_post_repetition')
      return [
        {
          key: 'repetition',
          name: name('topNounShare'),
          value: share(values?.repetitionShare),
          band: values && band('atMost', formatShare(values.repetitionShareWarnAbove)),
        },
        {
          key: 'relevance',
          name: name('titleNounsInBody'),
          value: share(values?.titleRelevance),
          band: values && band('atLeast', formatShare(values.titleRelevanceWarnBelow)),
        },
      ]
    },
    composition: (reading) => {
      const values = valuesOf(reading, 'composition')
      return [
        { key: 'chars', name: name('charCount'), value: amount('chars', values?.charCount) },
        { key: 'photos', name: name('photoCount'), value: amount('photos', values?.photoCount) },
        {
          key: 'types',
          name: name('blockTypes'),
          value: amount('types', values?.distinctBlockTypes),
          band: values && band('above', amount('types', values.distinctBlockTypesWarnAtOrBelow)),
        },
        {
          key: 'sentence',
          name: name('sentenceLength'),
          value: sentence(values?.averageSentenceLength),
        },
      ]
    },
  }

  return (
    <section aria-labelledby={headingId} className={className}>
      <Typography variant="label" as="h3" id={headingId}>
        {t('quality.post.heading', { ns: 'posts' })}
      </Typography>
      {measurement ? (
        <>
          <dl className="mt-2 grid grid-cols-1 gap-4 sm:grid-cols-3">
            {measurement.readings.filter(isPostMetric).map((reading) => (
              <div key={reading.metric}>
                <dt
                  className={typographyStyles({
                    variant: 'body',
                    className: 'text-content-primary flex flex-wrap items-center gap-2',
                  })}
                >
                  <span>{qualityMetricName(reading.metric)}</span>
                  <VerdictBadge verdict={reading.verdict} />
                </dt>
                {/* Below its minimum a metric names the minimum instead of a number it cannot
                    compare yet — the same line ①'s brief shows (QUAL-12, QUAL-36). */}
                {reading.verdict === 'below_minimum' ? (
                  <dd
                    className={typographyStyles({
                      variant: 'body',
                      className: 'text-content-secondary mt-1',
                    })}
                  >
                    {belowMinimumLine(reading.minimum, reading.publishedCount)}
                  </dd>
                ) : (
                  LINES[reading.metric](reading).map((line) => (
                    <dd
                      key={line.key}
                      className={typographyStyles({ variant: 'body', className: 'mt-1' })}
                    >
                      <span className="text-content-secondary">{line.name}</span>{' '}
                      <span className="text-content-primary tabular-nums">{line.value}</span>
                      {line.band && (
                        <span className="text-content-secondary tabular-nums"> · {line.band}</span>
                      )}
                    </dd>
                  ))
                )}
              </div>
            ))}
          </dl>
          <Typography variant="meta" as="p" className="text-content-secondary mt-3">
            {bandsAreOwnLine()}
          </Typography>
        </>
      ) : isError ? (
        <Typography variant="meta" as="p" className="text-content-secondary mt-2">
          {t('quality.post.failed', { ns: 'posts' })}{' '}
          <Button variant="ghost" onClick={refetch} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Typography>
      ) : (
        <Typography variant="meta" as="p" className="text-content-secondary mt-2">
          {t('quality.post.loading', { ns: 'posts' })}
        </Typography>
      )}
    </section>
  )
}

/** A badge only where the reading passed or warned: an absent value neither passes nor warns
 *  (QUAL-40), and a metric below its minimum says so in words. Colour is never the only signal —
 *  the badge carries its word (QUAL-37). */
function VerdictBadge({ verdict }: { verdict: QualityVerdict }) {
  const { t } = useTranslation('posts')
  if (verdict === 'over_band') return <Badge tone="warning">{t('quality.verdict.over_band')}</Badge>
  if (verdict === 'within_band')
    return <Badge tone="success">{t('quality.verdict.within_band')}</Badge>
  return null
}
