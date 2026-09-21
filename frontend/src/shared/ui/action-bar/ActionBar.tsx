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
  /** For a list's ONE add action. It has NO plane of its own at any width: no surface, no padding
   *  and no shadow behind the control, which floats over the rows on its own shadow at its natural
   *  width, against the right edge. A plane behind one button is a card holding one button however
   *  narrow it gets (THEME-13), and it covered a row of the list the whole time it was there. */
  | 'list'

/** The two docks share only where they STOP — a step above the phone tab bar, a step above the
 *  viewport edge from `sm:` — and part company over the plane. `always` spans the column and is a
 *  surface the committing controls sit on. `list` is the control itself: `w-fit` + `ml-auto` at
 *  every width, no surface, no padding, and the shadow moved onto what it holds so the button
 *  reads as floating over the rows rather than parked on a card. The caller keeps `mt-auto`, so a
 *  short list still pushes the dock to the bottom of the viewport rather than leaving it under
 *  the last row. A page whose gutters live on its blocks gives `list` its RIGHT gutter only
 *  (`mr-4 sm:mr-6 lg:mr-8`): an `mx-*` would replace `ml-auto` in the merge and drop the control
 *  back to the left edge. */
const DOCK_STYLES: Record<ActionBarDock, string> = {
  always:
    'bg-surface-highest bottom-dock-nav sm:pb-dock-b sticky z-20 mt-6 rounded-xl p-3 shadow-md sm:bottom-4 sm:p-4',
  list: 'bottom-dock-nav sticky z-20 mt-6 ml-auto w-fit sm:bottom-4 *:shadow-lg',
}

/** Docks a view's committing actions in the thumb's band instead of leaving them wherever the
 *  document flow put them (design-language §4.3). Lifted out of
 *  `features/review-model-experiment`, which hand-rolled this shape — the second slice needing it
 *  is what §1.1 says makes it a primitive.
 *
 *  It FLOATS clear of whatever is below it: a step above the phone tab bar (`bottom-dock-nav`),
 *  and a step above the viewport edge from `sm:` up where that bar does not exist. Resting on
 *  either one reads as a cut-off sheet, or as one two-storey slab of chrome, rather than as a
 *  dock hovering over the page. NN/g's bottom-sheet research is explicit that the extreme bottom
 *  is not the most reachable region, so that clearance satisfies both it and the platform
 *  tab-bar convention. `always` adds its own padding on top, one step tighter on a phone, where
 *  the bar can carry two rows of controls and the draft behind it is what the screen is for.
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
