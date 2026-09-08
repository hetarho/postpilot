import { Outlet } from '@tanstack/react-router'
import { Calculator, KeyRound, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { TabLinks, Typography, type TabLink, pageStyles } from '@/shared/ui'

/** The three operator surfaces, in one list so the row and the routes cannot drift — the same
 *  reason `VoiceLayout` keeps one `VOICE_TABS`. Each carries an icon and a short caption so the
 *  row compacts itself on a phone instead of scrolling (TabLinks' container mode).
 *
 *  The estimator combos have their own tab rather than a section under the catalog: the catalog
 *  runs to several hundred rows, so a section beneath it was a screen and a half of scrolling
 *  away from the tab row that named the page, and the operator setting a price tier is doing a
 *  different job from the one registering models. */
const ADMIN_TABS: readonly (Omit<TabLink, 'label' | 'shortLabel'> & {
  labelKey: 'accounts' | 'models' | 'estimator'
})[] = [
  { to: '/admin', labelKey: 'accounts', icon: Users },
  { to: '/admin/models', labelKey: 'models', icon: KeyRound },
  { to: '/admin/estimator', labelKey: 'estimator', icon: Calculator },
]

/** The frame of `/admin`: what the operator surface is, and the tab row over its three screens.
 *
 *  The tabs are addresses rather than state, so each one is bookmarkable and the browser's back
 *  button moves between them. The master guard lives once on the parent route — and every
 *  procedure under both tabs is refused server-side anyway, so a client that reached this frame
 *  by other means still reads nothing. */
export function AdminLayout() {
  const { t } = useTranslation('plans')

  return (
    <main className={pageStyles()}>
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
