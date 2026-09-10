import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { clsx } from 'clsx'
import { typographyStyles } from '@/shared/ui'

export interface NavDestination {
  to: string
  label: string
  /** The phone bar's caption when four full labels would not fit across the bottom edge. */
  shortLabel?: string
  icon: ComponentType<{ className?: string }>
}

/** Where the row is drawn. `header` and `row` are the laptop's two stacked bands, `rail` is either
 *  desk rail, `phone` is the bottom bar. */
export type NavShape = 'header' | 'row' | 'rail' | 'phone'
/** Which level the row IS, which is what decides its plane walk: the primary level sits on
 *  `surface-lowest` and the group level one step up on `surface-recessed` (THEME-26, THEME-38). */
export type NavLevel = 'primary' | 'group'

const shapeStyles: Record<NavShape, string> = {
  header: 'inline-flex min-h-11 min-w-11 shrink-0 items-center gap-2 rounded-md px-3',
  row: 'inline-flex min-h-11 min-w-11 shrink-0 items-center gap-2 rounded-md px-4',
  rail: 'flex min-h-11 items-center gap-3 rounded-md px-4',
  phone: 'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 px-1',
}

/** One step up from the level's own plane under the pointer, one further for the destination the
 *  user is on. The phone bar is not a rail — it floats over the page on `surface-raised` — so its
 *  current state is carried by weight and colour alone. */
const planeStyles: Record<NavLevel, { rest: string; current: string }> = {
  primary: {
    rest: 'hover:bg-surface-recessed active:bg-surface-recessed hover:text-link-fg-hover',
    current: 'bg-surface-base text-link-fg-current font-medium', // style-escape: current navigation emphasis
  },
  group: {
    rest: 'hover:bg-surface-base active:bg-surface-base hover:text-link-fg-hover',
    current: 'bg-surface-raised text-link-fg-current font-medium', // style-escape: current navigation emphasis
  },
}
const phoneStyles = {
  rest: 'active:bg-row-bg-active',
  current: 'text-link-fg-current font-medium', // style-escape: current navigation emphasis
}

/** The one renderer behind every navigation row in the shell, so the two levels cannot drift into
 *  two different controls (THEME-38: the second level is drawn with the first level's row shape).
 *
 *  Selection arrives one of two ways and both are needed: the primary level passes `current`,
 *  resolved from the ACTUAL matched route ids because `/voices` belongs to 글 without sharing its
 *  address, while the group level leaves it undefined and lets the router mark a destination
 *  current on its descendants too (`/templates/new` is still 글 템플릿). */
export function NavLinks({
  shape,
  level,
  destinations,
  current,
}: {
  shape: NavShape
  level: NavLevel
  destinations: readonly NavDestination[]
  current?: string
}) {
  const plane = shape === 'phone' ? phoneStyles : planeStyles[level]
  const className = typographyStyles({
    variant: 'label',
    className: clsx('text-link-fg whitespace-nowrap', shapeStyles[shape]),
  })
  return destinations.map((destination) => {
    // Both branches carry the same state when selection is computed here: a Link decides
    // `activeProps` from its own href, which would leave 글 unmarked on /voices.
    const computed =
      current === undefined
        ? undefined
        : {
            className: current === destination.to ? plane.current : plane.rest,
            'aria-current': current === destination.to ? ('page' as const) : undefined,
          }
    return (
      <Link
        key={destination.to}
        to={destination.to}
        aria-label={destination.label}
        className={className}
        activeOptions={{ exact: false }}
        activeProps={computed ?? { className: plane.current, 'aria-current': 'page' }}
        inactiveProps={computed ?? { className: plane.rest }}
      >
        <destination.icon aria-hidden="true" className="size-5 shrink-0" />
        {shape === 'phone' ? (destination.shortLabel ?? destination.label) : destination.label}
      </Link>
    )
  })
}
