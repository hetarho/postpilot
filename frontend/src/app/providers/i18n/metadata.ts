import i18next from 'i18next'
import { applyDocumentMetadata } from '@/shared/lib'
import type { Locale } from './locale'

/** The app's default document metadata: the title and description every route starts from.
 *
 *  It writes through `shared/lib`'s single transactional writer and DISCARDS the undo, because
 *  this is the baseline rather than a scoped override — there is nothing above it to restore to.
 *  A route that needs its own metadata (`pages/about`) applies its own transaction on top and
 *  restores this baseline for the current locale when it leaves. */
export function applyDocumentLocale(locale: Locale, target: Document = document): void {
  target.documentElement.lang = locale
  const t = i18next.getFixedT(locale, 'common')
  applyDocumentMetadata(
    { title: t('metadata.title'), meta: { description: t('metadata.description') } },
    target,
  )
}
