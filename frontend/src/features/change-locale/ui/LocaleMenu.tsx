import { useState } from 'react'
import { Languages } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { isLocale, type Locale } from '@/shared/lib/localization'
import { Menu } from '@/shared/ui'
import { changeLocale } from '../model/changeLocale'

export function LocaleMenu() {
  const { i18n, t } = useTranslation('common')
  const currentLocale: Locale = isLocale(i18n.resolvedLanguage) ? i18n.resolvedLanguage : 'ko'
  const [announcement, setAnnouncement] = useState('')

  const onChange = async (locale: Locale) => {
    await changeLocale(locale)
    // Announced in the NEW language: the person who asked for English should hear English.
    setAnnouncement(
      t('locale.changed', { language: t(`locale.${locale}`, { lng: locale }), lng: locale }),
    )
  }

  return (
    <div className="shrink-0">
      <Menu<Locale>
        label={t('locale.label')}
        value={currentLocale}
        options={[
          // Autonyms in both locales: the label a user can read is their own language's name.
          { value: 'ko', label: t('locale.ko') },
          { value: 'en', label: t('locale.en') },
        ]}
        onChange={(locale) => void onChange(locale)}
        triggerIcon={<Languages aria-hidden="true" className="size-4" />}
      />
      {/* Mounted before it speaks (§4.3): a live region inserted with its text announces nothing. */}
      <span className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </div>
  )
}
