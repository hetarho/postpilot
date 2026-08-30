import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** Docks a view's committing actions in the thumb's band instead of leaving them wherever the
 *  document flow put them (design-language §4.3). Lifted out of
 *  `features/review-model-experiment`, which hand-rolled this shape — the second slice needing it
 *  is what §1.1 says makes it a primitive.
 *
 *  It sticks ABOVE the phone tab bar (`bottom-nav`), and from `sm:` up — where that bar does not
 *  exist — it floats a step clear of the bottom edge instead of sitting on it: a rounded card
 *  flush against the viewport edge reads as a cut-off sheet rather than as a floating dock. The
 *  padding does the rest: NN/g's bottom-sheet research is explicit that the extreme bottom is not
 *  the most reachable region, so a docked bar with real padding is the shape that satisfies both
 *  that and the platform tab-bar convention.
 *
 *  ONE DOCK PER SCROLLER. Two of these in the same scroll container stick to the same offset and
 *  the later one in DOM order paints over the earlier one — both are opaque. A section that lives
 *  inside a page which already docks must put its action in flow instead. */
export function ActionBar({
  children,
  className,
  ariaLabel,
}: {
  children: ReactNode
  className?: string
  ariaLabel?: string
}) {
  return (
    <div
      aria-label={ariaLabel}
      className={twMerge(
        'bg-surface-highest bottom-nav sm:pb-dock-b sticky z-20 mt-6 rounded-xl p-4 shadow-md sm:bottom-4',
        className,
      )}
    >
      {children}
    </div>
  )
}
