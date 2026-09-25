import { useTranslation } from 'react-i18next'
import {
  QUALITY_METRICS,
  absentValueLabel,
  bandsAreOwnLine,
  belowMinimumLine,
  formatMeasure,
  formatShare,
  formatShareOrAbsent,
  qualityMetricName,
  qualityValuesOf,
  useAccountQuality,
  type QualityMetricId,
  type QualityReading,
} from '@/entities/quality'
import type { ContentLanguage } from '@/shared/api'
import { Badge, Button, Checkbox, Toggletip, Typography, typographyStyles } from '@/shared/ui'

interface QualityRuleChoicesProps {
  ownerId: string
  slug: string
  /** The post's target language: each rule text is rendered in it, so it keys the read. */
  targetLanguage: ContentLanguage
  /** The ticks the form holds, in catalogue order. */
  value: readonly QualityMetricId[]
  /** Reports the whole next set; the brief's 저장 saves it with the rest (POST-89). */
  onChange: (next: QualityMetricId[]) => void
  /** A running job, a published post or a save in flight: the boxes hold still, and the tips
   *  stay readable. */
  disabled: boolean
}

const measure = (value: number | undefined) =>
  value === undefined ? absentValueLabel() : formatMeasure(value)

/** The brief's rows over the account's 발행됨 posts, one per metric in its four states (POST-81).
 *  Only an over-band row offers a tick, which adds that metric's rule text to the next run. The
 *  rows are a control of the run-options form: a tick reports the next set and saves nothing, and
 *  the enqueue reads what the brief's 저장 saved. The client mirrors no number and judges nothing:
 *  every value, edge and verdict is the server's. */
export function QualityRuleChoices({
  ownerId,
  slug,
  targetLanguage,
  value,
  onChange,
  disabled,
}: QualityRuleChoicesProps) {
  const { t } = useTranslation(['posts', 'common'])
  const { quality, isError, isFetching, refetch } = useAccountQuality(ownerId, slug, targetLanguage)

  const toggle = (metric: QualityMetricId, on: boolean) => {
    const chosen = new Set(value)
    if (on) chosen.add(metric)
    else chosen.delete(metric)
    // The whole set, in catalogue order: a stored tick with no box now is kept, and the server
    // ignores it while its metric is within band.
    onChange(QUALITY_METRICS.filter((id) => chosen.has(id)))
  }

  // The value(s) a row shows beside the name. M4's three context values are ②'s, not the brief's.
  const values = (reading: QualityReading): string => {
    switch (reading.metric) {
      case 'title_saturation':
        return formatShareOrAbsent(qualityValuesOf(reading, 'title_saturation')?.share)
      case 'cross_post_phrases':
        return formatShareOrAbsent(qualityValuesOf(reading, 'cross_post_phrases')?.share)
      case 'in_post_repetition': {
        const found = qualityValuesOf(reading, 'in_post_repetition')
        return [
          t('qualityRules.values.repetition', {
            ns: 'posts',
            value: formatShareOrAbsent(found?.repetitionShare),
          }),
          t('qualityRules.values.relevance', {
            ns: 'posts',
            value: formatShareOrAbsent(found?.titleRelevance),
          }),
        ].join(' · ')
      }
      case 'composition':
        return t('qualityRules.values.blockTypes', {
          ns: 'posts',
          value: measure(qualityValuesOf(reading, 'composition')?.distinctBlockTypes),
        })
    }
  }

  // What the metric counts, why this row appeared, and the exact sentence ticking adds. Plain
  // text, because the tip mirrors it into a live region.
  const tip = (reading: QualityReading): string => {
    const count = reading.publishedCount
    const why = (() => {
      switch (reading.metric) {
        case 'title_saturation': {
          const found = qualityValuesOf(reading, 'title_saturation')
          return t('qualityRules.why.title_saturation', {
            ns: 'posts',
            count,
            value: formatShareOrAbsent(found?.share),
            edge: found ? formatShare(found.shareWarnAbove) : absentValueLabel(),
          })
        }
        case 'cross_post_phrases': {
          const found = qualityValuesOf(reading, 'cross_post_phrases')
          return t('qualityRules.why.cross_post_phrases', {
            ns: 'posts',
            count,
            value: formatShareOrAbsent(found?.share),
            edge: found ? formatShare(found.shareWarnAbove) : absentValueLabel(),
          })
        }
        case 'in_post_repetition': {
          const found = qualityValuesOf(reading, 'in_post_repetition')
          return t('qualityRules.why.in_post_repetition', {
            ns: 'posts',
            count,
            repetition: formatShareOrAbsent(found?.repetitionShare),
            relevance: formatShareOrAbsent(found?.titleRelevance),
            repetitionEdge: found
              ? formatShare(found.repetitionShareWarnAbove)
              : absentValueLabel(),
            relevanceEdge: found ? formatShare(found.titleRelevanceWarnBelow) : absentValueLabel(),
          })
        }
        case 'composition': {
          const found = qualityValuesOf(reading, 'composition')
          return t('qualityRules.why.composition', {
            ns: 'posts',
            count,
            value: measure(found?.distinctBlockTypes),
            edge: found ? formatMeasure(found.distinctBlockTypesWarnAtOrBelow) : absentValueLabel(),
          })
        }
      }
    })()
    const what = t(`qualityRules.what.${reading.metric}`, { ns: 'posts' })
    return `${what} ${why} ${t('qualityRules.adds', { ns: 'posts' })} “${reading.ruleText}”`
  }

  const row = (id: QualityMetricId) => {
    const name = qualityMetricName(id)
    // A metric missing from the answer is absent, never a missing row (POST-81).
    const reading = quality?.readings.find((candidate) => candidate.metric === id)
    const verdict = reading?.verdict ?? 'absent'
    if (reading && verdict === 'over_band') {
      return (
        <div key={id} className="flex items-start gap-2">
          <label
            className={typographyStyles({
              variant: 'label',
              className: 'flex min-h-11 min-w-0 flex-1 items-center gap-3',
            })}
          >
            <Checkbox
              checked={value.includes(id)}
              disabled={disabled}
              onChange={(event) => toggle(id, event.target.checked)}
            />
            <span className="min-w-0 break-words">
              {name} {values(reading)}
            </span>
          </label>
          {/* Beside the label, never inside it: a press inside a <label> would tick the box. */}
          <Toggletip label={t('qualityRules.explain', { ns: 'posts', metric: name })}>
            {tip(reading)}
          </Toggletip>
        </div>
      )
    }
    return (
      <div
        key={id}
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 flex-wrap items-center gap-x-3 gap-y-1',
        })}
      >
        <span>{name}</span>
        {reading && verdict === 'within_band' && (
          <>
            <span>{values(reading)}</span>
            <Badge tone="success">{t('quality.verdict.within_band', { ns: 'posts' })}</Badge>
          </>
        )}
        {reading && verdict === 'below_minimum' && (
          <span>{belowMinimumLine(reading.minimum, reading.publishedCount)}</span>
        )}
        {verdict === 'absent' && <span>{absentValueLabel()}</span>}
      </div>
    )
  }

  return (
    <div>
      <Typography variant="label" as="p">
        {t('qualityRules.heading', { ns: 'posts' })}
      </Typography>
      {quality ? (
        <>
          <div className="mt-2 grid grid-cols-1 gap-3">{QUALITY_METRICS.map(row)}</div>
          <Typography variant="meta" as="p" className="text-content-secondary mt-2">
            {bandsAreOwnLine()} {t('qualityRules.source', { ns: 'posts' })}
          </Typography>
        </>
      ) : isError ? (
        <Typography variant="meta" as="p" className="text-content-secondary mt-2">
          {t('qualityRules.failed', { ns: 'posts' })}{' '}
          <Button variant="ghost" onClick={refetch} pending={isFetching}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Typography>
      ) : (
        <Typography variant="meta" as="p" className="text-content-secondary mt-2">
          {t('qualityRules.loading', { ns: 'posts' })}
        </Typography>
      )}
    </div>
  )
}
