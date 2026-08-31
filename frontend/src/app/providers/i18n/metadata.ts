import i18next from 'i18next'
import type { Locale } from './locale'

export function applyDocumentLocale(locale: Locale, target: Document = document): void {
  target.documentElement.lang = locale
  const t = i18next.getFixedT(locale, 'common')
  target.title = t('metadata.title')

  let description = target.querySelector<HTMLMetaElement>('meta[name="description"]')
  if (!description) {
    description = target.createElement('meta')
    description.name = 'description'
    target.head.append(description)
  }
  description.content = t('metadata.description')
}
