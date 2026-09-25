import i18next from 'i18next'
import { formatNumber, formatPercent } from '@/shared/lib'
import type { QualityMetricId } from './types'

// The words and numbers every quality surface shares — ②'s row and ①'s brief say a metric, a
// value and a minimum the same way, so each lives here once.

export function qualityMetricName(id: QualityMetricId): string {
  return i18next.t(`quality.metric.${id}`, { ns: 'posts' })
}

/** A share in [0,1] as a percentage. One fraction digit, so a value just past its edge never
 *  rounds onto the edge it crossed and reads as equal to it. */
export function formatShare(value: number): string {
  return formatPercent(value, undefined, { maximumFractionDigits: 1 })
}

/** A count or an average, at most one fraction digit. */
export function formatMeasure(value: number): string {
  return formatNumber(value, undefined, { maximumFractionDigits: 1 })
}

/** A metric the account has too few 발행됨 posts for names its minimum (QUAL-12): the same line
 *  on ② and in ①'s brief (QUAL-36). */
export function belowMinimumLine(minimum: number, publishedCount: number): string {
  return i18next.t('quality.belowMinimum', { ns: 'posts', minimum, count: publishedCount })
}

/** Every band is the product's own, and each screen carrying one says so (QUAL-6). */
export function bandsAreOwnLine(): string {
  return i18next.t('quality.bandsOwn', { ns: 'posts' })
}

/** A value that could not be computed, which is not zero (QUAL-40). */
export function absentValueLabel(): string {
  return i18next.t('quality.absent', { ns: 'posts' })
}

/** A share, or 측정할 수 없어요 for one the server could not compute (QUAL-40). */
export function formatShareOrAbsent(value: number | undefined): string {
  return value === undefined ? absentValueLabel() : formatShare(value)
}
