import { Link } from '@tanstack/react-router'
import { twMerge } from 'tailwind-merge'

export interface TabLink {
  to: string
  label: string
}

/** A tab row whose tabs are ADDRESSES. `SegmentedControl` is the same shape driven by `onChange`,
 *  so it cannot give a tab a back button, a bookmark, or a modifier-click — a set of routed
 *  panels needs real links (design-language §1.1 corollary).
 *
 *  It is a `nav` of links, not `role="tablist"`: the ARIA tab pattern describes panels swapped in
 *  place, and announcing navigation as tab selection would tell a screen reader the page stayed
 *  put when the address changed. The current tab is marked `aria-current="page"` (§9).
 *
 *  Like `SegmentedControl` it scrolls horizontally rather than wrapping or crushing its labels:
 *  five Korean labels outgrow 328px of content long before their English equivalents would, and a
 *  clipped tab looks like a feature that does not exist. `overscroll-x-contain` keeps a swipe that
 *  reaches the end from chaining to the page or the browser's back gesture (§4.4). */
export function TabLinks({
  items,
  ariaLabel,
  className,
}: {
  items: readonly TabLink[]
  ariaLabel: string
  className?: string
}) {
  return (
    <nav
      aria-label={ariaLabel}
      className={twMerge(
        'bg-surface-recessed flex min-h-11 gap-1 overflow-x-auto overscroll-x-contain rounded-md p-1',
        className,
      )}
    >
      {items.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          // Exact: every tab is a sibling address under one pathless layout, so a prefix match
          // would leave 말투 (/voice) marked current on all four of the others.
          activeOptions={{ exact: true }}
          // `px-4` pays for `min-h-11`: it sets only the height, and '말투' is two Hangul at 14px —
          // a 28x44 target without the padding (§4.1, §4.2).
          className="text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active inline-flex min-h-11 flex-1 shrink-0 basis-auto items-center justify-center rounded-sm px-4 text-sm whitespace-nowrap transition-colors"
          activeProps={{
            className: 'bg-surface-raised text-content-primary shadow-sm',
            'aria-current': 'page',
          }}
        >
          {item.label}
        </Link>
      ))}
    </nav>
  )
}
