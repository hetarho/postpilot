import { Monitor, Moon, Palette, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Menu } from '@/shared/ui'
import type { ThemePreference } from '@/shared/lib'
import { useThemeController } from '../model/theme-controller'

/** The trigger names the SUBJECT, not the state: one palette, at every preference. A trigger that
 *  swapped glyph with the stored value had to say "theme" with a monitor on System, which reads as
 *  a display setting rather than as the appearance control, and the same shape then meant three
 *  different things across sessions. Each preference wears its own glyph inside the open menu,
 *  where a row stands for one choice alone, and the stored one stays announced on the closed
 *  trigger through `triggerDescription`. */
export function ThemeMenu() {
  const { t } = useTranslation('common')
  const { preference, setPreference } = useThemeController()

  return (
    <Menu<ThemePreference>
      label={t('theme.label')}
      triggerDescription={t('theme.current', { theme: t(`theme.${preference}`) })}
      value={preference}
      options={[
        { value: 'system', label: t('theme.system'), icon: Monitor },
        { value: 'light', label: t('theme.light'), icon: Sun },
        { value: 'dark', label: t('theme.dark'), icon: Moon },
      ]}
      onChange={setPreference}
      triggerIcon={<Palette aria-hidden="true" className="size-4" />}
    />
  )
}
