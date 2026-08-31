import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import {
  activeLocale,
  formatDateTime,
  formatMicroUsd,
  localeFromLanguageTag,
  type Locale,
} from '@/shared/lib/localization'
import { resolveLocale } from './locale'
import { applyDocumentLocale } from './metadata'
import { defaultNS, RESOURCE_NAMESPACES, resources } from './resources'

let configured = false

/** The three formats the catalogs use for machine values the server sends.
 *
 *  They exist because the server must not guess the reader's currency notation or timezone: it
 *  sends micro-USD integers and RFC3339 instants, and the browser turns them into text here. Both
 *  take the value as a STRING, because that is what a failure detail's params always are —
 *  i18next's built-in `datetime` would need a Date and cannot be used from that path.
 *
 *  Registered on the formatter service rather than through `interpolation.format`: i18next 26
 *  dropped that option from its typed init surface. */
function registerFormatters(): void {
  const formatter = i18next.services.formatter
  if (!formatter) return
  const localeOf = (lng: string | undefined) => localeFromLanguageTag(lng ?? '') ?? activeLocale()
  formatter.add('microusd', (value, lng) => formatMicroUsd(value as string | number, localeOf(lng)))
  formatter.add('instant', (value, lng) => formatDateTime(value as string, localeOf(lng)))
  // A tier arrives as its stored name (`max`), and a refusal has to name it the way the badge
  // does. Resolved through the catalog rather than through entities/plan so the i18n provider
  // keeps no dependency on a domain slice.
  formatter.add('plan', (value, lng) =>
    i18next.getFixedT(lng ?? null, 'plans')(`tier.${String(value)}`, {
      defaultValue: String(value),
    }),
  )
}

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
      //
      // The formatters exist because the server sends machine values — micro-USD integers and
      // RFC3339 instants — and must not guess the reader's currency notation or timezone. A
      // catalog writes `{{limit, microusd}}` and the browser resolves it locally.
      interpolation: { escapeValue: false },
      react: { useSuspense: false },
      returnNull: false,
    })
    registerFormatters()
  } else if (i18next.language !== locale) {
    void i18next.changeLanguage(locale)
  }

  syncDocument(locale)
  return i18next
}

export { resolveLocale }
export type { Locale }
