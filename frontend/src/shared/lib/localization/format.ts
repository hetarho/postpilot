import i18next from 'i18next'
import { activeLocale, intlLocale, type Locale } from './locale'

const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const DAY_MS = 24 * HOUR_MS
const WEEK_MS = 7 * DAY_MS

export function formatNumber(
  value: number | bigint,
  locale: Locale = activeLocale(),
  options: Intl.NumberFormatOptions = {},
): string {
  return new Intl.NumberFormat(intlLocale(locale), options).format(value)
}

export function formatPercent(
  value: number,
  locale: Locale = activeLocale(),
  options: Intl.NumberFormatOptions = {},
): string {
  return formatNumber(value, locale, {
    style: 'percent',
    maximumFractionDigits: 0,
    ...options,
  })
}

export function formatDate(value: string | Date, locale: Locale = activeLocale()): string {
  const date = typeof value === 'string' ? new Date(value) : value
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(intlLocale(locale), { dateStyle: 'medium' }).format(date)
}

export function formatDateTime(value: string | Date, locale: Locale = activeLocale()): string {
  const date = typeof value === 'string' ? new Date(value) : value
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(intlLocale(locale), {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

/** Human-readable time for a list row. Thresholds intentionally preserve the existing product
 * behavior; only the active locale changes the wording. */
export function formatRelativeTime(
  iso: string,
  now: Date = new Date(),
  locale: Locale = activeLocale(),
): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''

  const elapsed = Math.max(0, now.getTime() - at.getTime())
  if (elapsed < MINUTE_MS) return i18next.getFixedT(locale, 'common')('time.justNow')

  const relative = new Intl.RelativeTimeFormat(intlLocale(locale), { numeric: 'always' })
  if (elapsed < HOUR_MS) return relative.format(-Math.floor(elapsed / MINUTE_MS), 'minute')
  if (elapsed < DAY_MS) return relative.format(-Math.floor(elapsed / HOUR_MS), 'hour')
  if (elapsed < WEEK_MS) return relative.format(-Math.floor(elapsed / DAY_MS), 'day')
  return formatDate(at, locale)
}
