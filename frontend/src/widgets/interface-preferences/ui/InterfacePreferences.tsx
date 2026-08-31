import { useId } from 'react'
import { SlidersHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { LocaleSelect } from '@/features/change-locale'
import { ThemeSelector } from '@/features/change-theme'
import { Popover } from '@/shared/ui'

/** One compact public surface for browser-local interface preferences. Keeping both segmented
 * switches inside the popover makes their current values visible at a glance without making the
 * 320px app header carry two controls beside its session action, and keeps every open state
 * app-drawn — no OS-native option list breaks the surface (design-language §7). */
export function InterfacePreferences() {
  const { t } = useTranslation('common')
  const localeHeadingId = useId()
  const themeHeadingId = useId()
  return (
    <Popover
      label={t('interfacePreferences.label')}
      triggerLabel={
        <>
          <SlidersHorizontal aria-hidden="true" className="size-7" />
          <span className="hidden sm:inline">{t('interfacePreferences.trigger')}</span>
        </>
      }
      triggerClassName="px-3 sm:px-4"
      placement="below"
    >
      {() => (
        <div className="grid gap-4">
          <section aria-labelledby={localeHeadingId}>
            <h2 id={localeHeadingId} className="text-content-secondary mb-1 text-sm font-medium">
              {t('locale.label')}
            </h2>
            <LocaleSelect />
          </section>
          <section aria-labelledby={themeHeadingId}>
            <h2 id={themeHeadingId} className="text-content-secondary mb-1 text-sm font-medium">
              {t('theme.label')}
            </h2>
            <ThemeSelector />
          </section>
        </div>
      )}
    </Popover>
  )
}
