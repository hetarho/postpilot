import { useTranslation } from 'react-i18next'
import type { ThemePreference } from '@/shared/lib'
import { SegmentedControl } from '@/shared/ui'
import { useThemeController } from '../model/theme-controller'

/** The same preference as `ThemeMenu`, drawn as a bounded switch instead of a menu.
 *
 *  A menu cannot open INSIDE a popover: the account panel is height-capped and scrolls, so the
 *  panel a menu would open is clipped by the surface that holds its trigger. A switch states all
 *  three choices in the row it already occupies and needs nothing to open. */
export function ThemeSegments({ size }: { size?: 'default' | 'compact' }) {
  const { t } = useTranslation('common')
  const { preference, setPreference } = useThemeController()

  return (
    <SegmentedControl<ThemePreference>
      size={size}
      ariaLabel={t('theme.label')}
      value={preference}
      options={[
        { value: 'system', label: t('theme.system') },
        { value: 'light', label: t('theme.light') },
        { value: 'dark', label: t('theme.dark') },
      ]}
      onChange={setPreference}
    />
  )
}
