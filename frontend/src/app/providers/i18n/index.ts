import type { Locale } from '@/shared/lib/localization'
import { configureI18n } from './configure'
import { resolveLocale } from './locale'
import { resources } from './resources'

/** Initializes bundled resources synchronously. Call in the entry path before createRoot. */
export function initializeI18n(locale: Locale = resolveLocale()) {
  return configureI18n(resources, locale)
}

export { resolveLocale }
export type { Locale }
