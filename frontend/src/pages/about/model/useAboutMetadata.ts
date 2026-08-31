import { useEffect } from 'react'
import i18next from 'i18next'
import { useTranslation } from 'react-i18next'
import { activeLocale, applyDocumentMetadata, type Locale } from '@/shared/lib'

/** `/about`'s canonical path. Fixed rather than read from the router: the canonical URL of the
 *  marketing page is a content decision, and a locale change must not move it (plan 15 — no
 *  locale-prefixed URLs). */
const ABOUT_PATH = '/about'

/** The Open Graph locale tag for each supported UI locale. */
const OG_LOCALES: Record<Locale, string> = { ko: 'ko_KR', en: 'en_US' }

/** Applies `/about`'s localized document metadata for as long as the page is mounted, and hands
 *  the head back when it leaves.
 *
 *  It lives in the page slice, not in the i18n provider, because these strings and this canonical
 *  path belong to one route. The provider owns only the app-wide default.
 *
 *  Cleanup is two steps, and both are needed. The transaction's own undo removes the Open Graph
 *  and canonical tags this route created — that is what stops marketing metadata leaking into
 *  `/login`. Reasserting the app default afterwards fixes the title and description, because the
 *  undo restores the bytes captured at APPLY time and the locale may have changed since: without
 *  it, switching to English on `/about` and then leaving would put the Korean title back. */
export function useAboutMetadata(): void {
  const { t, i18n } = useTranslation('marketing')
  const locale = (i18n.resolvedLanguage ?? i18n.language) as string

  useEffect(() => {
    const title = t('metadata.title')
    const description = t('metadata.description')
    const ogLocale = OG_LOCALES[activeLocale()]
    // The current origin, not a build-time constant: the same bundle is served from the
    // production host and from a local dev server, and a canonical pointing at the wrong one is
    // worse than none.
    const url = `${window.location.origin}${ABOUT_PATH}`

    const restore = applyDocumentMetadata({
      title,
      meta: { description },
      property: {
        'og:type': 'website',
        'og:title': title,
        'og:description': description,
        'og:locale': ogLocale,
        'og:url': url,
      },
      link: { canonical: url },
    })

    return () => {
      restore()
      // Reassert through the same shared writer the provider uses, for the locale that is current
      // NOW. i18next is the language source here rather than the provider's helper, which lives in
      // the app layer and a page may not import.
      const fallback = i18next.getFixedT(null, 'common')
      applyDocumentMetadata({
        title: fallback('metadata.title'),
        meta: { description: fallback('metadata.description') },
      })
    }
  }, [locale, t])
}
