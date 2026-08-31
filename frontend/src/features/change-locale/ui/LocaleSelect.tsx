import { useId, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { isLocale, type Locale } from '@/shared/lib/localization'
import { Select } from '@/shared/ui'
import { changeLocale } from '../model/changeLocale'

export function LocaleSelect() {
  const id = useId()
  const { i18n, t } = useTranslation('common')
  const currentLocale: Locale = isLocale(i18n.resolvedLanguage) ? i18n.resolvedLanguage : 'ko'
  const [announcement, setAnnouncement] = useState('')

  const onChange = async (event: ChangeEvent<HTMLSelectElement>) => {
    const locale = event.target.value
    if (!isLocale(locale) || locale === currentLocale) return
    await changeLocale(locale)
    setAnnouncement(
      t('locale.changed', { language: t(`locale.${locale}`, { lng: locale }), lng: locale }),
    )
  }

  return (
    <div className="shrink-0">
      <label htmlFor={id} className="sr-only">
        {t('locale.label')}
      </label>
      <Select
        id={id}
        value={currentLocale}
        onChange={(event) => void onChange(event)}
        aria-label={t('locale.label')}
        className="w-auto min-w-24 pr-9"
      >
        <option value="ko">{t('locale.ko')}</option>
        <option value="en">{t('locale.en')}</option>
      </Select>
      <span className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </span>
    </div>
  )
}
