import { useTranslation } from 'react-i18next'
import { LocaleMenu, LocaleSegments } from '@/features/change-locale'
import { ThemeMenu, ThemeSegments } from '@/features/change-theme'
import { Typography } from '@/shared/ui'

/** The one locale/theme composition, in the two shapes the shells need.
 *
 * `icons` is the header pair on a surface with no account behind it — login, signup, `/about` —
 * where each preference is its own icon-triggered app-drawn menu and no open state is ever
 * OS-native (design-language §7).
 *
 * `rows` is the same two preferences inside the account panel, where the authenticated shell now
 * keeps them (owner decision 2026-09-22): a signed-in person has one place for everything about
 * their session, and the header keeps two fewer controls at 320px. A menu cannot open inside that
 * panel — it is height-capped and scrolls — so each preference states its choices in place.
 *
 * Either shape is one labelled group, so assistive tech hears "인터페이스 환경설정" once rather
 * than two unrelated controls. */
export function InterfacePreferences({ layout = 'icons' }: { layout?: 'icons' | 'rows' }) {
  const { t } = useTranslation('common')
  if (layout === 'rows')
    return (
      <div role="group" aria-label={t('interfacePreferences.label')} className="grid gap-3">
        {/* The key is the PREFERENCE, never its translated name: keying on the label remounted
            both rows the moment the locale changed, which threw away the switch's travelling
            plane mid-press and made the language row look like it had no animation at all. */}
        {[
          { id: 'theme', label: t('theme.label'), control: <ThemeSegments size="compact" /> },
          { id: 'locale', label: t('locale.label'), control: <LocaleSegments size="compact" /> },
        ].map(({ id, label, control }) => (
          <div key={id} className="grid gap-1">
            <Typography variant="meta" as="p">
              {label}
            </Typography>
            {control}
          </div>
        ))}
      </div>
    )
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
