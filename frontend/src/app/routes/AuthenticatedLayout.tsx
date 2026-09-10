import { Link, Outlet, useMatches, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { Logo } from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { CreditBadge } from '@/widgets/credit-badge'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'
import { NavLinks, type NavDestination, type NavShape } from './NavLinks'
import { currentDestination, DESTINATIONS } from './navigation'

/** One guarded shell, one document scroller, and one destination registry.
 * Phone: the primary level is the bottom bar; laptop: its own band under the brand bar; desk: the
 * outer rail. The group level is the second band or the inner rail and belongs to
 * `ContentGroupLayout`, which knows which group is current — the shell must not learn that.
 *
 * The three chrome rows have named heights (`--spacing-bar`, `--spacing-primaryrow`,
 * `--spacing-subnav`) so `--spacing-chrome` — what every page-level sticky element clears — can be
 * composed from them instead of from a literal that drifts when a row's padding changes. */
export function AuthenticatedLayout() {
  const { t } = useTranslation('nav')
  const { user } = useSession()
  const navigate = useNavigate()
  const current = useMatches({
    select: (matches) => currentDestination(matches.map((m) => m.routeId)),
  })
  const destinations: NavDestination[] = DESTINATIONS.filter(
    (d) => !d.masterOnly || user?.plan === 'master',
  ).map((d) => ({
    to: d.to,
    label: t(d.labelKey),
    shortLabel: 'phoneLabelKey' in d ? t(d.phoneLabelKey) : undefined,
    icon: d.icon,
  }))
  const primary = (shape: NavShape) => (
    <NavLinks shape={shape} level="primary" destinations={destinations} current={current} />
  )

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <header className="sm:min-h-header sm:sticky sm:top-0 sm:z-20">
        {/* A wrapping cluster preserves every digit and the 44 px controls at 320 px.
            There is only one balance, theme, locale and account at any width. */}
        <div className="bg-surface-raised sm:min-h-bar lg:min-h-header flex min-h-14 flex-wrap items-center justify-between gap-x-2 px-4 py-2 sm:px-6 sm:py-1 lg:py-0">
          <Link
            to="/posts"
            className="inline-flex min-h-11 shrink-0 items-center px-2"
            aria-label={t('home')}
          >
            <Logo className="h-6" />
          </Link>
          <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2">
            <CreditBadge />
            <InterfacePreferences />
            <AccountMenu
              onLoggedOut={() => {
                endSession()
                void navigate({ to: '/login', replace: true })
              }}
            />
          </div>
        </div>
        {/* The laptop's primary band. It is chrome, so it runs edge to edge on its own plane and
            scrolls horizontally rather than crushing its targets. */}
        <nav
          className="bg-surface-lowest sm:h-primaryrow hidden items-center gap-1 overflow-x-auto overscroll-x-contain px-4 sm:flex sm:px-6 lg:hidden"
          aria-label={t('primary')}
        >
          {primary('header')}
        </nav>
      </header>
      <div className="flex flex-1 flex-col lg:flex-row">
        <aside className="bg-surface-lowest lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:w-52 lg:flex-col lg:overflow-y-auto lg:px-3 lg:py-4">
          <nav className="flex flex-col gap-2" aria-label={t('primary')}>
            {primary('rail')}
          </nav>
        </aside>
        <div className="pb-nav flex min-w-0 flex-1 flex-col sm:pb-0">
          <Outlet />
        </div>
      </div>
      <nav
        className="bg-surface-raised pb-safe-b fixed inset-x-0 bottom-0 z-30 flex gap-2 shadow-lg sm:hidden"
        aria-label={t('primary')}
      >
        {primary('phone')}
      </nav>
    </div>
  )
}
