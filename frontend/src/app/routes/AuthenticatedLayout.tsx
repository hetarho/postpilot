import { Link, Outlet, useMatches, useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { clsx } from 'clsx'
import { useSession } from '@/entities/session'
import { Button, CollapsibleRail, Logo, Menu, PromoStage, useRailToggle } from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { CreditBadge } from '@/widgets/credit-badge'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { endSession } from '../model/end-session'
import { NavLinks, type NavDestination, type NavShape } from './NavLinks'
import { RailNav, type RailItem } from './RailNav'
import {
  CONTENT_GROUPS,
  CONTENT_GROUP_LABELS,
  currentDestination,
  currentGroup,
  currentGroupDestination,
  DESTINATIONS,
} from './navigation'

/** One guarded shell, one document scroller, and one destination registry — and, since
 *  2026-09-22, one place that knows BOTH navigation levels.
 *
 *  The group level used to belong to `ContentGroupLayout`, further down the tree, which is why
 *  the desk had two rails and the group's row sat on the page rather than in the chrome. Reading
 *  the group off `DESTINATIONS` instead lets the desk hold one sidebar that opens the group under
 *  the destination that owns it, and lets every other width put the group where the owner asked
 *  for it: the middle of the brand row, between the logo and the account.
 *
 *  Phone: the primary level is the bottom bar. Laptop: its own band under the brand row. Desk:
 *  the one rail, which folds to its glyphs and hands its reopen to the brand row while folded.
 *
 *  The chrome rows have named heights (`--spacing-bar`, `--spacing-primaryrow`) so
 *  `--spacing-chrome` — what every page-level sticky element clears — can be composed from them
 *  instead of from a literal that drifts when a row's padding changes. */
export function AuthenticatedLayout() {
  const { t } = useTranslation('nav')
  const { user } = useSession()
  const navigate = useNavigate()
  const rail = useRailToggle()
  const routeIds = useMatches({ select: (matches) => matches.map((m) => m.routeId) })
  const current = currentDestination(routeIds)
  const group = currentGroup(routeIds)
  const immersive = useMatches({
    select: (matches) => matches.some((m) => m.pathname === '/plans'),
  })
  // A workspace inside a group asks for the second level to be ABSENT (CLIP-37).
  const groupHidden = useMatches({
    select: (matches) => matches.some((m) => m.staticData.groupNav === 'hidden'),
  })
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const stage = useRouterState({ select: (state) => state.location.search.stage })
  const from = useRouterState({ select: (state) => state.location.search.from })

  const destinations: NavDestination[] = DESTINATIONS.filter(
    (d) => !d.masterOnly || user?.plan === 'master',
  ).map((d) => ({
    to: d.to,
    label: t(d.labelKey),
    icon: d.icon,
  }))
  const primary = (shape: NavShape) => (
    <NavLinks shape={shape} destinations={destinations} current={current} />
  )

  const openGroup = group && !groupHidden ? group : undefined
  const groupLabel = openGroup ? t(CONTENT_GROUP_LABELS[openGroup]) : undefined
  const groupDestinations = openGroup
    ? CONTENT_GROUPS[openGroup].map((d) => ({
        to: d.to,
        label: t(d.labelKey),
        icon: d.icon,
        search: openGroup === 'models' ? { stage } : undefined,
      }))
    : []
  const groupCurrent = openGroup ? currentGroupDestination(openGroup, pathname, from).to : undefined

  // The rail's one list: every primary destination, with the open group's own destinations
  // directly under the row that opened them.
  const railItems: RailItem[] = DESTINATIONS.filter(
    (d) => !d.masterOnly || user?.plan === 'master',
  ).flatMap((d) => [
    { to: d.to, label: t(d.labelKey), icon: d.icon, level: 'primary', current: current === d.to },
    ...(d.group && d.group === openGroup
      ? groupDestinations.map((g) => ({
          ...g,
          level: 'group' as const,
          current: groupCurrent === g.to,
        }))
      : []),
  ])

  return (
    <div
      data-plans-shell={immersive || undefined}
      className="bg-surface-base text-content-primary relative isolate flex min-h-full flex-col"
    >
      {immersive && <PromoStage viewport />}
      {/* `relative z-20` at EVERY width, not from `sm:` up: below `sm:` the header is static, a
          static box ignores `z-index`, and a page that makes its own stacking context — `/plans`
          rises into place with a transform — then paints over the account panel opened from here
          (owner report 2026-09-22). An open control inside it lifts the whole header above the
          group's own row, the way the group row already lifts itself. */}
      <header
        className={clsx(
          'sm:min-h-header relative z-20 has-[[aria-expanded=true]]:z-40 sm:sticky sm:top-0',
          immersive ? 'bg-surface-raised/70 backdrop-blur-xl' : 'bg-surface-raised',
        )}
      >
        {/* Three parts, so the group can hold the middle: brand, the group the address is under,
            and the session. A wrapping cluster preserves every digit and the 44px controls at
            320px. There is only one balance and one account at any width. */}
        <div className="sm:min-h-bar lg:min-h-header flex min-h-14 flex-wrap items-center gap-x-2 px-4 py-2 sm:px-6 sm:py-1 lg:py-0">
          <div className="flex shrink-0 items-center gap-1">
            <Link
              to="/posts"
              className="inline-flex min-h-11 shrink-0 items-center px-2"
              aria-label={t('home')}
            >
              <Logo className="h-6" />
            </Link>
            {/* Both halves of one control, in ONE place beside the mark (owner decision
                2026-09-22). It stays put whether the rail is open or folded — a control that
                moved when pressed had to be looked for again — and it exists only where a rail
                is drawn at all. */}
            <Button
              variant="ghost"
              size="icon"
              className="hidden lg:inline-flex"
              aria-expanded={rail.open}
              aria-label={rail.open ? t('sidebar.close') : t('sidebar.open')}
              onClick={rail.toggle}
            >
              {rail.open ? (
                <PanelLeftClose aria-hidden="true" className="size-4" />
              ) : (
                <PanelLeftOpen aria-hidden="true" className="size-4" />
              )}
            </Button>
          </div>
          {/* The group, in the middle of the chrome rather than at the top of the page (owner
              decision 2026-09-22). It is the desk's rail at every other width, so it stops where
              the rail starts. `min-w-0` with the flexible sides is what keeps a long group name
              truncating instead of pushing the account off the row. */}
          {openGroup && (
            <nav aria-label={groupLabel} className="flex min-w-0 flex-1 justify-center lg:hidden">
              <Menu
                label={groupLabel!}
                value={groupCurrent!}
                options={groupDestinations.map((d) => ({
                  value: d.to,
                  label: d.label,
                  icon: d.icon,
                }))}
                onChange={(to) =>
                  void navigate({ to, search: openGroup === 'models' ? { stage } : undefined })
                }
                triggerLabel={
                  groupDestinations.find((d) => d.to === groupCurrent)?.label ?? groupLabel!
                }
                triggerIcon={(() => {
                  const Icon = groupDestinations.find((d) => d.to === groupCurrent)?.icon ?? Logo
                  return <Icon aria-hidden="true" className="size-5 shrink-0" />
                })()}
                triggerClassName="text-link-fg-current max-w-full"
              />
            </nav>
          )}
          {/* `ml-auto` at every width: the middle is empty at the desk, where the group is part
              of the rail, and without it the session cluster sat against the brand. */}
          <div className="ml-auto flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
            <CreditBadge />
            {/* The desk keeps both preferences as their own header controls, where there is room
                for them; every narrower width finds them in the account panel instead. */}
            <div className="hidden lg:flex">
              <InterfacePreferences />
            </div>
            <AccountMenu
              preferences={<InterfacePreferences layout="rows" />}
              onLoggedOut={() => {
                endSession()
                void navigate({ to: '/login', replace: true })
              }}
            />
          </div>
        </div>
        {/* The tablet navigation shares the brand row's background. The header paints one
            surface, including its translucent backdrop on /plans, so there is no dark band
            between the brand and the page. Targets still scroll horizontally when needed. */}
        <nav
          className="sm:h-primaryrow hidden items-center gap-1 overflow-x-auto overscroll-x-contain px-4 sm:flex sm:px-6 lg:hidden"
          aria-label={t('primary')}
        >
          {primary('header')}
        </nav>
      </header>
      <div className="flex flex-1 flex-col lg:flex-row">
        <CollapsibleRail
          toggle={rail}
          width="14rem"
          className={clsx(
            'lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:flex-col',
            immersive ? 'bg-surface-lowest/70 backdrop-blur-xl' : 'bg-surface-lowest',
          )}
          boxClassName="lg:overflow-y-auto lg:px-3 lg:py-4"
          collapsedBoxClassName="lg:py-4"
        >
          {(collapsed) => (
            <nav
              className={clsx('flex flex-col gap-1.5', collapsed && 'items-center')}
              aria-label={t('primary')}
            >
              <RailNav items={railItems} collapsed={collapsed} />
            </nav>
          )}
        </CollapsibleRail>
        <div className="pb-nav flex min-w-0 flex-1 flex-col sm:pb-0">
          <Outlet />
        </div>
      </div>
      <nav
        className={clsx(
          'pb-safe-b fixed inset-x-0 bottom-0 z-30 flex gap-2 shadow-lg sm:hidden',
          immersive ? 'bg-surface-raised/70 backdrop-blur-xl' : 'bg-surface-raised',
        )}
        aria-label={t('primary')}
      >
        {primary('phone')}
      </nav>
    </div>
  )
}
