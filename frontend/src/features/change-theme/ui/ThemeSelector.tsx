import { useId, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { isThemePreference } from '@/shared/lib'
import { Select } from '@/shared/ui'
import { useThemeController } from '../model/theme-controller'

export function ThemeSelector() {
  const id = useId()
  const { t } = useTranslation('common')
  const { preference, setPreference } = useThemeController()

  const onChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const next = event.target.value
    if (!isThemePreference(next) || next === preference) return
    setPreference(next)
  }

  return (
    <div className="shrink-0">
      <label htmlFor={id} className="sr-only">
        {t('theme.label')}
      </label>
      <Select
        id={id}
        value={preference}
        onChange={onChange}
        aria-label={t('theme.label')}
        className="w-full min-w-24 pr-9"
      >
        <option value="system">{t('theme.system')}</option>
        <option value="light">{t('theme.light')}</option>
        <option value="dark">{t('theme.dark')}</option>
      </Select>
    </div>
  )
}
