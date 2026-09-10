import { Outlet } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
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
 *  `chrome-subnav` is what tells everything inside the group that the sticky chrome got taller. */
export function ContentGroupLayout({ group }: { group: keyof typeof CONTENT_GROUPS }) {
  const { t } = useTranslation('nav')
  const label = t(group === 'writing' ? 'writingGroup' : 'videoGroup')
  const destinations = CONTENT_GROUPS[group].map(({ labelKey, to, icon }) => ({
    to,
    label: t(labelKey),
    icon,
  }))
  return (
    <div className="chrome-subnav flex min-w-0 flex-1 flex-col lg:flex-row">
      <nav
        aria-label={label}
        className="bg-surface-recessed sm:top-header h-subnav sticky top-0 z-10 flex items-center gap-1 overflow-x-auto overscroll-x-contain px-4 sm:px-6 lg:hidden"
      >
        <NavLinks shape="row" level="group" destinations={destinations} />
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
