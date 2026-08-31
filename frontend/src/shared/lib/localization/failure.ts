import i18next from 'i18next'
import type { AppFailure } from '@/shared/api'
import { activeLocale, type Locale } from './locale'

/** Formats only the stable, allowlisted failure contract. Provider/backend prose is deliberately
 * absent from this boundary and must never become the primary user-facing explanation. */
export function formatAppFailure(
  failure: AppFailure | undefined,
  locale: Locale = activeLocale(),
): string {
  if (!failure) return ''
  const translate = i18next.getFixedT(locale, 'errors') as unknown as (
    key: string,
    options: Readonly<Record<string, string>>,
  ) => string
  return translate(failure.reason, failure.params)
}
