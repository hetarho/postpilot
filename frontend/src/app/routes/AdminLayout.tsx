import { Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Calculator, Coins, Gift, KeyRound, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Typography, chipStyles, type TabLink, pageStyles } from '@/shared/ui'

/** The operator surfaces, in one list so the row and the routes cannot drift — the same
 *  reason `VoiceLayout` keeps one `VOICE_TABS`. Each carries an icon and a short caption so the
 *  row compacts itself on a phone instead of scrolling (TabLinks' container mode).
 *
 *  The estimator combos have their own tab rather than a section under the catalog: the catalog
 *  runs to several hundred rows, so a section beneath it was a screen and a half of scrolling
 *  away from the tab row that named the page, and the operator setting a price tier is doing a
 *  different job from the one registering models. */
const ADMIN_TABS: readonly (Omit<TabLink, 'label' | 'shortLabel'> & {
  labelKey: 'accounts' | 'models' | 'estimator' | 'vouchers' | 'costs'
})[] = [
  { to: '/admin', labelKey: 'accounts', icon: Users },
  { to: '/admin/models', labelKey: 'models', icon: KeyRound },
  { to: '/admin/estimator', labelKey: 'estimator', icon: Calculator },
  { to: '/admin/vouchers', labelKey: 'vouchers', icon: Gift },
  { to: '/admin/costs', labelKey: 'costs', icon: Coins },
]

/** The frame of `/admin`: what the operator surface is, and the tab row over its screens.
 *
 *  The tabs are addresses rather than state, so each one is bookmarkable and the browser's back
 *  button moves between them. The master guard lives once on the parent route — and every
 *  procedure under both tabs is refused server-side anyway, so a client that reached this frame
 *  by other means still reads nothing. */
export function AdminLayout() {
  const { t } = useTranslation('plans')
  const pathname = useRouterState({ select: (state) => state.location.pathname }).replace(
    /\/+$/u,
    '',
  )

  return (
    <main className={pageStyles({ width: 'wide' })}>
      <Typography variant="largeTitle">{t('admin.title')}</Typography>
      {/* Guide: the operator's screens as the same sideways chip row every hub uses, each chip
          led by its glyph. */}
      <nav
        aria-label={t('admin.title')}
        className="-mx-4 mt-4 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:px-0"
      >
        {ADMIN_TABS.map(({ labelKey, to, icon: Icon }) => {
          const current = pathname === to
          return (
            <Link
              key={to}
              to={to}
              aria-current={current ? 'page' : undefined}
              className={chipStyles({ current })}
            >
              {Icon && <Icon aria-hidden="true" className="size-4" />}
              {t(`admin.tab.${labelKey}`)}
            </Link>
          )
        })}
      </nav>
      <Outlet />
    </main>
  )
}
