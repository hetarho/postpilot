import { useTranslation } from 'react-i18next'
import type { ThemePreference } from '@/shared/lib'
import { SegmentedControl } from '@/shared/ui'
import { useThemeController } from '../model/theme-controller'

export function ThemeSelector() {
  const { t } = useTranslation('common')
  const { preference, setPreference } = useThemeController()

  const onChange = (next: ThemePreference) => {
    if (next === preference) return
    setPreference(next)
  }

  return (
    <div className="shrink-0">
      <SegmentedControl<ThemePreference>
        value={preference}
        options={[
          { value: 'system', label: t('theme.system') },
          { value: 'light', label: t('theme.light') },
          { value: 'dark', label: t('theme.dark') },
        ]}
        onChange={onChange}
        ariaLabel={t('theme.label')}
      />
    </div>
  )
}
