import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isLocale, type Locale } from '@/shared/lib/localization'
import { SegmentedControl } from '@/shared/ui'
import { changeLocale } from '../model/changeLocale'

/** The same choice as `LocaleMenu`, drawn as a bounded switch for a surface a menu cannot open
 *  inside of — see `ThemeSegments`. Both autonyms are visible at once here, which is the one
 *  thing a language switch most wants: a reader who cannot read the current language can still
 *  find their own. */
export function LocaleSegments({ size }: { size?: 'default' | 'compact' }) {
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
    <>
      <SegmentedControl<Locale>
        size={size}
        ariaLabel={t('locale.label')}
        value={currentLocale}
        options={[
          { value: 'ko', label: t('locale.ko') },
          { value: 'en', label: t('locale.en') },
        ]}
        onChange={(locale) => void onChange(locale)}
      />
      {/* Mounted before it speaks (§4.3): a live region inserted with its text announces nothing. */}
      <span className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </>
  )
}
