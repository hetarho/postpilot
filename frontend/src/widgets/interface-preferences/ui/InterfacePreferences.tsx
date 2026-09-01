import { useTranslation } from 'react-i18next'
import { LocaleMenu } from '@/features/change-locale'
import { ThemeMenu } from '@/features/change-theme'

/** The one locale/theme composition, mounted by the login, authenticated, and `/about` shells.
 * Each preference is its own icon-triggered app-drawn menu — the theme trigger wears the current
 * preference, so both values stay legible from the closed header — and no open state is ever
 * OS-native (design-language §7). The pair is one labelled group so assistive tech hears "인터페이스
 * 환경설정" once rather than two unrelated buttons. */
export function InterfacePreferences() {
  const { t } = useTranslation('common')
  return (
    <div
      role="group"
      aria-label={t('interfacePreferences.label')}
      className="flex shrink-0 items-center gap-2"
    >
      <ThemeMenu />
      <LocaleMenu />
    </div>
  )
}
