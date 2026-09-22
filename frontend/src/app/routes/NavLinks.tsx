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

/** Where the row is drawn. `header` is the laptop's own navigation band under the brand row,
 *  `phone` is the bottom bar. The desk rail is `RailNav`, which draws both levels at once. */
export type NavShape = 'header' | 'phone'

const shapeStyles: Record<NavShape, string> = {
  // The tablet row fills the header's full primary band and sits flush on the page, so its
  // current destination reads as a tab that the page continues, not as a button floating in the
  // band: only the top corners are rounded.
  header: 'inline-flex min-h-11 min-w-11 shrink-0 items-center gap-2 rounded-t-md px-3',
  phone: 'flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-1 px-1',
}

/** One step up from the band's plane under the pointer, one further for the destination the user
 *  is on. The phone bar is not a band — it floats over the page on `surface-raised` — so its
 *  current state is carried by weight and colour alone. */
const planeStyles = {
  header: {
    rest: 'hover:bg-surface-recessed active:bg-surface-recessed hover:text-link-fg-hover',
    current: 'bg-surface-base text-link-fg-current font-medium', // style-escape: current navigation emphasis
  },
  phone: {
    rest: 'active:bg-row-bg-active',
    current: 'text-link-fg-current font-medium', // style-escape: current navigation emphasis
  },
} as const

/** The primary level below the desk: a band of tabs on the laptop, the bottom bar on a phone.
 *
 *  Selection is resolved from the ACTUAL matched route ids by the caller, because `/voices`
 *  belongs to 글 without sharing its address, so a `Link` deciding from its own href would leave
 *  글 unmarked there. */
export function NavLinks({
  shape,
  destinations,
  current,
}: {
  shape: NavShape
  destinations: readonly NavDestination[]
  current?: string
}) {
  const plane = planeStyles[shape]
  const className = typographyStyles({
    variant: 'label',
    className: clsx('text-link-fg whitespace-nowrap', shapeStyles[shape]),
  })
  return destinations.map((destination) => {
    const isCurrent = current === destination.to
    return (
      <Link
        key={destination.to}
        to={destination.to}
        aria-label={destination.label}
        className={clsx(className, isCurrent ? plane.current : plane.rest)}
        // The caller has already decided: `exact` stops the router's own prefix match and both
        // props carry the one answer, which is how 글 stays marked on /voices — an address it
        // does not own — and how no second destination lights up under a shared prefix.
        activeOptions={{ exact: true, includeSearch: false }}
        activeProps={{ 'aria-current': isCurrent ? 'page' : undefined }}
        inactiveProps={{ 'aria-current': isCurrent ? 'page' : undefined }}
      >
        <destination.icon aria-hidden="true" className="size-5 shrink-0" />
        {shape === 'phone' ? (destination.shortLabel ?? destination.label) : destination.label}
      </Link>
    )
  })
}
