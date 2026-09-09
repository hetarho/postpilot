import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** Which shape the dock takes above the phone. It docks at EVERY width either way (THEME-24):
 *  a list long enough to scroll pushes its one add action below the fold otherwise, and that is
 *  the action the screen exists for. */
export type ActionBarDock =
  /** Spans the column. For a scroller whose CONTENT is tall no matter how wide the window is:
   *  the editor's draft, an experiment's two candidate columns. What makes the bar stick there is
   *  the distance between the thing and the control that commits it, not the thumb. */
  | 'always'
  /** Shrinks to its contents and settles against the right edge from `sm:` up. For a bar carrying
   *  a list's ONE add action: full-bleed in the thumb's band on a phone, and above it a bar only
   *  as wide as its button — which is what keeps it from being the full-width card holding one
   *  left-aligned button that the retired phone-only rule was written against. */
  | 'list'

/** `list` is `always` plus the shrink: same sticky plane, corner and shadow at every width, and
 *  from `sm:` up `w-fit` + `ml-auto` take it down to the width of what it holds. The caller keeps
 *  `mt-auto`, so a short list still pushes the bar to the bottom of the viewport rather than
 *  leaving it under the last row. */
const DOCK_BASE =
  'bg-surface-highest bottom-dock-nav sm:pb-dock-b sticky z-20 mt-6 rounded-xl p-3 shadow-md sm:bottom-4 sm:p-4'

const DOCK_STYLES: Record<ActionBarDock, string> = {
  always: DOCK_BASE,
  list: `${DOCK_BASE} sm:ml-auto sm:w-fit`,
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
