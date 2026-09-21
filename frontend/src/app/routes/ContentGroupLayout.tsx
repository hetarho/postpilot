import { Outlet, useMatches, useNavigate, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Menu } from '@/shared/ui'
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
 *  The band is ONE row holding ONE control: the name of the destination the address is under —
 *  the group's home, 내 글 or 내 영상, by default — and pressing that name opens the group. It was
 *  first the rail's links as a scrolling row of pills, which read as a row of buttons rather than
 *  as a menu and never said where the owner was (owner decision 2026-09-19); then a name on the
 *  left with a menu button at the far right, which is two things for one job and puts the way
 *  into the group as far from the name as the row allows (owner decision 2026-09-21). The rail on
 *  the desk keeps every destination as a link: there the row is a column with room for all of
 *  them.
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
        className="bg-surface-base sm:top-header h-subnav sticky top-0 z-10 flex items-center justify-center px-4 pt-2 sm:px-6 lg:hidden"
      >
        {/* Where the owner IS and the way to the rest of the group, in one control: the current
            destination's glyph and name in the current colour, with a chevron. It keeps no plane
            at rest, so it still reads as the place's name rather than as a button parked in the
            chrome. It is CENTRED in the row rather than pulled onto the page gutter: the row
            holds nothing else, so a name parked at the left edge read as the start of a list that
            was not there (owner decision 2026-09-21). The band itself sits on the page's plane,
            opaque only so the page cannot scroll through it. */}
        <Menu
          label={label}
          value={current.to}
          options={destinations.map((d) => ({ value: d.to, label: d.label }))}
          onChange={(to) => void navigate({ to })}
          triggerLabel={current.label}
          triggerIcon={<current.icon aria-hidden="true" className="size-5 shrink-0" />}
          triggerClassName="text-link-fg-current max-w-full"
        />
      </nav>
      {/* Narrower, tighter and closer-packed than the primary rail beside it: the density is the
          second half of what tells the two levels apart, the plane being the first (THEME-38). */}
      <aside className="bg-surface-recessed lg:top-header lg:h-sidebar hidden shrink-0 lg:sticky lg:flex lg:w-44 lg:flex-col lg:overflow-y-auto lg:px-2 lg:py-3">
        <nav aria-label={label} className="flex flex-col gap-1">
          <NavLinks shape="rail" level="group" destinations={destinations} />
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </div>
    </div>
  )
}
