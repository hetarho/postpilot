import type { ComponentType } from 'react'
import { Link } from '@tanstack/react-router'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface TabLink {
  to: string
  label: string
  /** Compact-mode caption. Korean labels ('기존 글 가져오기') outgrow an evenly divided 320px row,
   *  so the compact tab may carry a shorter word; the full label stays the wide-mode text. */
  shortLabel?: string
  /** Enables the icon-compact narrow mode — only when EVERY item in the row provides one, since a
   *  row mixing icon tabs with text tabs would read as two different controls. */
  icon?: ComponentType<{ className?: string }>
  /** Route params for a `to` with dynamic segments, such as the voice a tab row belongs to. */
  params?: Record<string, string>
}

/** A tab row whose tabs are ADDRESSES. `SegmentedControl` is the same shape driven by `onChange`,
 *  so it cannot give a tab a back button, a bookmark, or a modifier-click — a set of routed
 *  panels needs real links (design-language §1.1 corollary).
 *
 *  It is a `nav` of links, not `role="tablist"`: the ARIA tab pattern describes panels swapped in
 *  place, and announcing navigation as tab selection would tell a screen reader the page stayed
 *  put when the address changed. The current tab is marked `aria-current="page"` (§9).
 *
 *  Text-only rows scroll horizontally rather than wrapping or crushing their labels: five Korean
 *  labels outgrow 328px of content long before their English equivalents would, and a clipped tab
 *  looks like a feature that does not exist. When every item carries an `icon`, the row instead
 *  switches shape with its own width (`@container`, §1.5): below `@tabs` (38rem) each tab stacks
 *  its icon over a compact caption and the five share
 *  the row evenly, so nothing is off-screen on a phone; from `@tabs` up it is today's text row.
 *  `overscroll-x-contain` keeps a swipe that reaches the end of the scrolling variant from
 *  chaining to the page or the browser's back gesture (§4.4). */
export function TabLinks({
  items,
  ariaLabel,
  className,
}: {
  items: readonly TabLink[]
  ariaLabel: string
  className?: string
}) {
  const compactCapable = items.every((item) => item.icon)
  return (
    <nav
      aria-label={ariaLabel}
      className={twMerge(
        'bg-surface-recessed @container flex min-h-11 gap-1 overflow-x-auto overscroll-x-contain rounded-md p-1',
        className,
      )}
    >
      {items.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          params={item.params}
          // In compact-capable rows the full label is the link's ONE accessible name: the two
          // visual captions are decoration that swaps with the container width, and without this
          // a non-CSS reader (or JSDOM) would hear both concatenated.
          aria-label={compactCapable ? item.label : undefined}
          // Exact: every tab is a sibling address under one layout, so a prefix match would leave
          // the first tab marked current on all of the others.
          activeOptions={{ exact: true }}
          // `px-4` pays for `min-h-11`: it sets only the height, and '말투' is two Hangul at 14px —
          // a 28x44 target without the padding (§4.1, §4.2).
          className={clsx(
            'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active inline-flex min-h-11 flex-1 items-center justify-center rounded-sm text-sm whitespace-nowrap transition-colors',
            compactCapable
              ? '@tabs:shrink-0 @tabs:basis-auto @tabs:flex-row @tabs:px-1 min-w-0 flex-col gap-0.5 px-1'
              : 'shrink-0 basis-auto px-4',
          )}
          activeProps={{
            className: 'bg-surface-raised text-content-primary shadow-sm',
            'aria-current': 'page',
          }}
        >
          {compactCapable && item.icon && (
            <item.icon aria-hidden="true" className="@tabs:hidden size-5 shrink-0" />
          )}
          {compactCapable ? (
            <>
              <span aria-hidden="true" className="@tabs:hidden text-xs leading-tight">
                {item.shortLabel ?? item.label}
              </span>
              <span aria-hidden="true" className="@tabs:inline hidden">
                {item.label}
              </span>
            </>
          ) : (
            item.label
          )}
        </Link>
      ))}
    </nav>
  )
}
