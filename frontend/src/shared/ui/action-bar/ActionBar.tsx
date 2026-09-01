import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** Docks a view's committing actions in the thumb's band instead of leaving them wherever the
 *  document flow put them (design-language §4.3). Lifted out of
 *  `features/review-model-experiment`, which hand-rolled this shape — the second slice needing it
 *  is what §1.1 says makes it a primitive.
 *
 *  It FLOATS clear of whatever is below it: a step above the phone tab bar (`bottom-dock-nav`),
 *  and a step above the viewport edge from `sm:` up where that bar does not exist. Resting the
 *  card on either one reads as a cut-off sheet, or as one two-storey slab of chrome, rather than
 *  as a dock hovering over the page. The padding does the rest: NN/g's bottom-sheet research is
 *  explicit that the extreme bottom is not the most reachable region, so a docked bar with real
 *  padding is the shape that satisfies both that and the platform tab-bar convention. It is one
 *  step tighter on a phone, where the bar can carry two rows of controls and the draft behind it
 *  is what the screen is for.
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
        'bg-surface-highest bottom-dock-nav sm:pb-dock-b sticky z-20 mt-6 rounded-xl p-3 shadow-md sm:bottom-4 sm:p-4',
        className,
      )}
    >
      {children}
    </div>
  )
}
