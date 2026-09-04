import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** How far up the widths the bar keeps its DOCK treatment — the sticky, rounded, shadowed card. */
export type ActionBarDock =
  /** Every width. For a scroller whose CONTENT is tall no matter how wide the window is: the
   *  editor's draft, an experiment's two candidate columns. What makes the bar stick there is the
   *  distance between the thing and the control that commits it, not the thumb — so it does not
   *  go away when the thumb does. */
  | 'always'
  /** The phone only. For a bar carrying a list's ONE add action. From `sm:` up the reach argument
   *  evaporates (§4.3), and what is left is a floating card holding a single left-aligned button
   *  over a half-empty page — which reads as debris, not as a dock. So above the phone the card
   *  dissolves and the bar becomes what it actually is: the list's last row, with its action
   *  spanning the column. */
  | 'phone'

/** `phone` is `always` minus the card, from `sm:` up. Each reset undoes exactly one thing the
 *  dock treatment does — the position, the plane, the corner, the shadow, the inset that held the
 *  contents off the card's edge — and `sm:mt-8` replaces the caller's `mt-auto`, so the bar sits
 *  under the last row instead of being pushed to the bottom of a half-empty viewport. */
const DOCK_STYLES: Record<ActionBarDock, string> = {
  always:
    'bg-surface-highest bottom-dock-nav sm:pb-dock-b sticky z-20 mt-6 rounded-xl p-3 shadow-md sm:bottom-4 sm:p-4',
  phone:
    'bg-surface-highest bottom-dock-nav sticky z-20 mt-6 rounded-xl p-3 shadow-md sm:static sm:mt-8 sm:rounded-none sm:bg-transparent sm:p-0 sm:shadow-none',
}

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
 *  inside a page which already docks must put its action in flow instead.
 *
 *  ONE INSTANCE PER ACTION, too, which is why `dock` is a prop and not a second call site. A
 *  trigger owns the overlay it opens, so a phone bar plus a desktop copy of the same trigger is
 *  two sheets waiting to be opened, and two elements the tests have to tell apart. */
export function ActionBar({
  children,
  className,
  ariaLabel,
  dock = 'always',
}: {
  children: ReactNode
  className?: string
  ariaLabel?: string
  dock?: ActionBarDock
}) {
  return (
    <div aria-label={ariaLabel} className={twMerge(DOCK_STYLES[dock], className)}>
      {children}
    </div>
  )
}
