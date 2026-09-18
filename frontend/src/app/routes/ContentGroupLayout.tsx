import { Outlet, useMatches, useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Menu as MenuIcon } from 'lucide-react'
import { Menu, typographyStyles } from '@/shared/ui'
import { NavLinks } from './NavLinks'
import { CONTENT_GROUPS } from './navigation'

/** The second level of navigation: chrome, not page content (THEME-38).
 *
 *  It is drawn twice because the two shapes sit in different places in the document — a band above
 *  the content below the desk, an inner rail beside it from `lg:` — and one element cannot be
 *  both. The band sticks to the top of the viewport: on a phone the header scrolls away, so it
 *  stops at `top-0`; from `sm:` the header is sticky and it stops under it. It cannot read
 *  `top-chrome` for that, since it is itself what that token measures.
 *
 *  The band is ONE row: the name of the destination the address is under — the group's home,
 *  내 글 or 내 영상, by default — and a menu control at its right holding every destination of the
 *  group with the current one checked. It used to be the rail's links laid out as a scrolling row
 *  of pills, which on a phone read as a row of buttons rather than as a menu, and gave no sense of
 *  where the owner was (owner decision 2026-09-19). The rail on the desk keeps every destination
 *  as a link: there the row is a column with room for all of them.
 *
 *  `chrome-subnav` is what tells everything inside the group that the sticky chrome got taller. */
export function ContentGroupLayout({ group }: { group: keyof typeof CONTENT_GROUPS }) {
  const { t } = useTranslation('nav')
  const navigate = useNavigate()
  // A workspace inside the group asks for the level to be ABSENT (CLIP-37): no row, no rail, and
  // no `chrome-subnav`, so a sticky element on that page clears only what is actually stuck.
  const hidden = useMatches({
    select: (matches) => matches.some((match) => match.staticData.groupNav === 'hidden'),
  })
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const label = t(group === 'writing' ? 'writingGroup' : 'videoGroup')
  const destinations = CONTENT_GROUPS[group].map(({ labelKey, to, icon }) => ({
    to,
    label: t(labelKey),
    icon,
  }))
  if (hidden)
    return (
      <div className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </div>
    )
  // The destination the address is under, resolved the way the rail's links mark themselves
  // current (a prefix match, so `/voices/one/rules` is still 말투); the group's home otherwise.
  const current =
    destinations.find((d) => pathname === d.to || pathname.startsWith(`${d.to}/`)) ??
    destinations[0]!
  return (
    <div className="chrome-subnav flex min-w-0 flex-1 flex-col lg:flex-row">
      <nav
        aria-label={label}
        className="bg-surface-base sm:top-header h-subnav sticky top-0 z-10 flex items-center justify-between gap-2 px-4 sm:px-6 lg:hidden"
      >
        {/* Where the owner IS: the current destination's icon and name, in the rail's current
            colour and no plane of its own — a name, not another button. The band itself sits on
            the page's plane, opaque only so the page cannot scroll through it. */}
        <span
          aria-current="page"
          className={typographyStyles({
            variant: 'label',
            className: 'text-link-fg-current inline-flex min-h-11 min-w-0 items-center gap-2 px-2',
          })}
        >
          <current.icon aria-hidden="true" className="size-5 shrink-0" />
          <span className="truncate">{current.label}</span>
        </span>
        <Menu
          label={label}
          value={current.to}
          options={destinations.map((d) => ({ value: d.to, label: d.label }))}
          onChange={(to) => void navigate({ to })}
          triggerIcon={<MenuIcon aria-hidden="true" className="size-5" />}
        />
      </nav>
      <aside className="bg-surface-recessed lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:w-48 lg:flex-col lg:overflow-y-auto lg:px-3 lg:py-4">
        <nav aria-label={label} className="flex flex-col gap-2">
          <NavLinks shape="rail" level="group" destinations={destinations} />
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </div>
    </div>
  )
}
