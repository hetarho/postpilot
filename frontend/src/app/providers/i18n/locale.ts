import {
  FALLBACK_LOCALE,
  localeFromLanguageTag,
  readLocaleOverride,
  type Locale,
} from '@/shared/lib/localization'

export interface LocaleResolutionInput {
  storage?: Pick<Storage, 'getItem'>
  navigatorLanguages?: readonly string[]
}

function browserLanguages(): readonly string[] {
  try {
    return typeof navigator === 'undefined' ? [] : navigator.languages
  } catch {
    return []
  }
}

export function resolveLocale(input: LocaleResolutionInput = {}): Locale {
  const stored = readLocaleOverride(input.storage)
  if (stored) return stored

  for (const language of input.navigatorLanguages ?? browserLanguages()) {
    const locale = localeFromLanguageTag(language)
    if (locale) return locale
  }
  return FALLBACK_LOCALE
}

export type { Locale }
