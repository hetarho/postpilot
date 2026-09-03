import { Outlet } from '@tanstack/react-router'
import { KeyRound, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { TabLinks, Typography, type TabLink } from '@/shared/ui'

/** The two operator surfaces, in one list so the row and the routes cannot drift — the same
 *  reason `VoiceLayout` keeps one `VOICE_TABS`. Both carry an icon and a short caption so the
 *  row compacts itself on a phone instead of scrolling (TabLinks' container mode). */
const ADMIN_TABS: readonly (Omit<TabLink, 'label' | 'shortLabel'> & {
  labelKey: 'accounts' | 'models'
})[] = [
  { to: '/admin', labelKey: 'accounts', icon: Users },
  { to: '/admin/models', labelKey: 'models', icon: KeyRound },
]

/** The frame of `/admin`: what the operator surface is, and the tab row over its two screens.
 *
 *  The tabs are addresses rather than state, so each one is bookmarkable and the browser's back
 *  button moves between them. The master guard lives once on the parent route — and every
 *  procedure under both tabs is refused server-side anyway, so a client that reached this frame
 *  by other means still reads nothing. */
export function AdminLayout() {
  const { t } = useTranslation('plans')

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <Typography variant="display">{t('admin.title')}</Typography>
      <TabLinks
        items={ADMIN_TABS.map(({ labelKey, ...tab }) => ({
          ...tab,
          label: t(`admin.tab.${labelKey}`),
          shortLabel: t(`admin.tabShort.${labelKey}`),
        }))}
        ariaLabel={t('admin.title')}
        className="mt-4"
      />
      <Outlet />
    </main>
  )
}
