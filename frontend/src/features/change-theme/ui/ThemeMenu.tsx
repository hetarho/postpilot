import type { ComponentType } from 'react'
import { Monitor, Moon, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { ThemePreference } from '@/shared/lib'
import { Menu } from '@/shared/ui'
import { useThemeController } from '../model/theme-controller'

/** The trigger tells the truth before the menu opens: it wears the icon of the STORED preference
 *  (System is the monitor even while resolving dark), because the control edits the preference,
 *  not the effective theme. */
const PREFERENCE_ICONS: Record<ThemePreference, ComponentType<{ className?: string }>> = {
  system: Monitor,
  light: Sun,
  dark: Moon,
}

export function ThemeMenu() {
  const { t } = useTranslation('common')
  const { preference, setPreference } = useThemeController()
  const TriggerIcon = PREFERENCE_ICONS[preference]

  return (
    <Menu<ThemePreference>
      label={t('theme.label')}
      triggerDescription={t('theme.current', { theme: t(`theme.${preference}`) })}
      value={preference}
      options={[
        { value: 'system', label: t('theme.system') },
        { value: 'light', label: t('theme.light') },
        { value: 'dark', label: t('theme.dark') },
      ]}
      onChange={setPreference}
      triggerIcon={<TriggerIcon aria-hidden="true" className="size-5" />}
    />
  )
}
