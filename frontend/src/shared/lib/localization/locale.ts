import i18next from 'i18next'

export const SUPPORTED_LOCALES = ['ko', 'en'] as const
export const LOCALE_STORAGE_KEY = 'postpilot.locale'

export type Locale = (typeof SUPPORTED_LOCALES)[number]

export const FALLBACK_LOCALE: Locale = 'ko'

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && SUPPORTED_LOCALES.includes(value as Locale)
}

export function localeFromLanguageTag(value: unknown): Locale | undefined {
  if (typeof value !== 'string') return undefined
  const primary = value.trim().split('-', 1)[0]?.toLowerCase()
  return isLocale(primary) ? primary : undefined
}

export function activeLocale(): Locale {
  return localeFromLanguageTag(i18next.resolvedLanguage ?? i18next.language) ?? FALLBACK_LOCALE
}

export function intlLocale(locale: Locale = activeLocale()): 'ko-KR' | 'en-US' {
  return locale === 'en' ? 'en-US' : 'ko-KR'
}

export function browserLocaleStorage(): Pick<Storage, 'getItem' | 'setItem'> | undefined {
  try {
    return globalThis.localStorage
  } catch {
    return undefined
  }
}

export function readLocaleOverride(
  storage: Pick<Storage, 'getItem'> | undefined = browserLocaleStorage(),
): Locale | undefined {
  try {
    const value = storage?.getItem(LOCALE_STORAGE_KEY)
    return isLocale(value) ? value : undefined
  } catch {
    return undefined
  }
}

export function writeLocaleOverride(
  locale: Locale,
  storage: Pick<Storage, 'setItem'> | undefined = browserLocaleStorage(),
): void {
  try {
    storage?.setItem(LOCALE_STORAGE_KEY, locale)
  } catch {
    // A denied storage API must not prevent the in-memory locale from changing.
  }
}
