import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isLocale, type Locale } from '@/shared/lib/localization'
import { SegmentedControl } from '@/shared/ui'
import { changeLocale } from '../model/changeLocale'

export function LocaleSelect() {
  const { i18n, t } = useTranslation('common')
  const currentLocale: Locale = isLocale(i18n.resolvedLanguage) ? i18n.resolvedLanguage : 'ko'
  const [announcement, setAnnouncement] = useState('')

  const onChange = async (locale: Locale) => {
    if (locale === currentLocale) return
    await changeLocale(locale)
    setAnnouncement(
      t('locale.changed', { language: t(`locale.${locale}`, { lng: locale }), lng: locale }),
    )
  }

  return (
    <div className="shrink-0">
      <SegmentedControl<Locale>
        value={currentLocale}
        options={[
          // Autonyms in both locales: the label a user can read is their own language's name.
          { value: 'ko', label: t('locale.ko') },
          { value: 'en', label: t('locale.en') },
        ]}
        onChange={(locale) => void onChange(locale)}
        ariaLabel={t('locale.label')}
      />
      <span className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </div>
  )
}
