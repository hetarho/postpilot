import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import { localeFromLanguageTag, type Locale } from '@/shared/lib/localization'
import { resolveLocale } from './locale'
import { applyDocumentLocale } from './metadata'
import { defaultNS, RESOURCE_NAMESPACES, resources } from './resources'

let configured = false

function syncDocument(language: string): void {
  if (typeof document === 'undefined') return
  const locale = localeFromLanguageTag(language)
  if (locale) applyDocumentLocale(locale)
}

/** Initializes bundled resources synchronously. Call in the entry path before createRoot. */
export function initializeI18n(locale: Locale = resolveLocale()) {
  if (!configured) {
    configured = true
    i18next.use(initReactI18next)
    i18next.on('languageChanged', syncDocument)
    void i18next.init({
      resources,
      lng: locale,
      fallbackLng: 'ko',
      supportedLngs: ['ko', 'en'],
      load: 'languageOnly',
      ns: RESOURCE_NAMESPACES,
      defaultNS,
      initAsync: false,
      // React owns output escaping. Letting i18next escape first would turn a literal `&` or
      // `<` in a filename/voice/model name into visible HTML entities after React escapes the
      // translated string a second time.
      interpolation: { escapeValue: false },
      react: { useSuspense: false },
      returnNull: false,
    })
  } else if (i18next.language !== locale) {
    void i18next.changeLanguage(locale)
  }

  syncDocument(locale)
  return i18next
}

export { resolveLocale }
export type { Locale }
