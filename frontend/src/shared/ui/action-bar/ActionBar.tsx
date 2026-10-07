import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** Both shapes stay docked at every width so long content cannot hide its committing action. */
export type ActionBarDock =
  /** A compact translucent surface around the controls, or their bounded composer. */
  | 'always'
  /** List add actions float at their natural width without a background plane. */
  | 'list'

/** Action groups fit their controls; a composer has readable space for its fields. */
type ActionBarWidth = 'actions' | 'content'

const DOCK_STYLES: Record<ActionBarDock, string> = {
  always: 'bottom-dock-nav sticky z-20 mx-auto mt-6 min-w-0 self-center rounded-xl p-2 shadow-md',
  list: 'bottom-dock-nav sticky z-20 mt-6 ml-auto w-fit max-w-full *:shadow-lg',
}

const WIDTH_STYLES: Record<ActionBarWidth, string> = {
  actions: 'w-fit max-w-full',
  content: 'w-full max-w-measure',
}

/** One committing dock per document scroller, clear of the viewport's safe edge (THEME-24).
 *  The stylesheet owns its glass surface, solid fallback and larger contained action targets.
 *  DOM-scoped sizing covers Button and buttonStyles links while portalled panels keep their
 *  ordinary sizing. Keep one mounted instance of every action at all viewport widths. */
export function ActionBar({
  children,
  className,
  ariaLabel,
  dock = 'always',
  width = 'actions',
}: {
  children: ReactNode
  className?: string
  ariaLabel?: string
  dock?: ActionBarDock
  width?: ActionBarWidth
}) {
  return (
    <div
      aria-label={ariaLabel}
      data-action-bar={dock}
      className={twMerge(DOCK_STYLES[dock], dock === 'always' && WIDTH_STYLES[width], className)}
    >
      {children}
    </div>
  )
}
