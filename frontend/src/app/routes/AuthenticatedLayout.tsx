import { Link, Outlet, useMatches, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { Logo, typographyStyles } from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { CreditBadge } from '@/widgets/credit-badge'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'
import { currentDestination, DESTINATIONS } from './navigation'

const linkStyles = {
  header: typographyStyles({
    variant: 'label',
    className:
      'text-link-fg hover:text-link-fg-hover active:bg-row-bg-active inline-flex min-h-11 min-w-11 items-center justify-center px-2 whitespace-nowrap',
  }),
  phone: typographyStyles({
    variant: 'label',
    className:
      'text-link-fg active:bg-row-bg-active flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 px-1',
  }),
  desk: typographyStyles({
    variant: 'label',
    className: 'text-link-fg flex min-h-11 items-center gap-3 rounded-md px-4',
  }),
}
type Destination = (typeof DESTINATIONS)[number]
function PrimaryLinks({
  shape,
  destinations,
  current,
}: {
  shape: keyof typeof linkStyles
  destinations: readonly Destination[]
  current?: string
}) {
  const { t } = useTranslation('nav')
  return destinations.map((destination) => {
    const selected = current === destination.to
    // Supply the same matched-group state in both Link branches: /voices must
    // highlight writing even though the writing entry itself leads to /posts.
    const state = {
      className: selected
        ? shape === 'desk'
          ? 'bg-surface-raised text-link-fg-current font-medium' // style-escape: current navigation emphasis
          : 'text-link-fg-current font-medium' // style-escape: current navigation emphasis
        : shape === 'desk'
          ? 'hover:bg-surface-base active:bg-surface-base hover:text-link-fg-hover'
          : '',
      'aria-current': selected ? ('page' as const) : undefined,
    }
    return (
      <Link
        key={destination.to}
        to={destination.to}
        aria-label={t(destination.labelKey)}
        className={linkStyles[shape]}
        activeProps={state}
        inactiveProps={state}
      >
        {shape !== 'header' && <destination.icon aria-hidden="true" className="size-5 shrink-0" />}
        {t(
          shape === 'phone' && 'phoneLabelKey' in destination
            ? destination.phoneLabelKey
            : destination.labelKey,
        )}
      </Link>
    )
  })
}

/** One guarded shell, one document scroller, and one destination registry.
 * Phone: bottom navigation; laptop: header row; desk: recessed left rail.
 * The header can wrap on a phone without shrinking the brand or any target.
 * The laptop's dedicated second row has a matching top-header token so sticky
 * progress and the desk rail never sit under chrome with an untracked height. */
export function AuthenticatedLayout() {
  const { t } = useTranslation('nav')
  const { user } = useSession()
  const navigate = useNavigate()
  const current = useMatches({
    select: (matches) => currentDestination(matches.map((m) => m.routeId)),
  })
  const destinations = DESTINATIONS.filter((d) => !d.masterOnly || user?.plan === 'master')

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <header className="bg-surface-raised sm:min-h-header flex min-h-14 flex-wrap items-center justify-between gap-x-2 px-4 py-2 sm:sticky sm:top-0 sm:z-20 sm:px-6 sm:py-1 lg:py-0">
        <Link
          to="/posts"
          className="inline-flex min-h-11 shrink-0 items-center px-2"
          aria-label={t('home')}
        >
          <Logo className="h-6" />
        </Link>
        {/* A wrapping cluster preserves every digit and the 44 px controls at 320 px.
            There is only one balance, theme, locale and account at any width. */}
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
        <nav
          className="hidden w-full items-center gap-2 sm:flex lg:hidden"
          aria-label={t('primary')}
        >
          <PrimaryLinks shape="header" destinations={destinations} current={current} />
        </nav>
      </header>
      <div className="flex flex-1 flex-col lg:flex-row">
        <aside className="bg-surface-recessed lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:w-60 lg:flex-col lg:overflow-y-auto lg:px-4 lg:py-4">
          <nav className="flex flex-col gap-2" aria-label={t('primary')}>
            <PrimaryLinks shape="desk" destinations={destinations} current={current} />
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
        <PrimaryLinks shape="phone" destinations={destinations} current={current} />
      </nav>
    </div>
  )
}
