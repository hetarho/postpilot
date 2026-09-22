import { type ReactNode } from 'react'
import { clsx } from 'clsx'
import { motion, useReducedMotion } from 'motion/react'
import type { RailToggle } from './useRailToggle'

/** A desk rail that folds down to its glyphs instead of leaving.
 *
 *  Folding is not closing (owner decision 2026-09-22): a destination stays one press away at
 *  either width, so the rail never has to hand its way back to somebody else's chrome, and the
 *  one control that folds it never leaves the screen. That is also why there is no presence to
 *  animate — the aside is always mounted and only its WIDTH moves, on `--duration-base` and
 *  `--ease-standard` as motion reads them, so it travels on the same clock as every CSS
 *  transition in the app. A reduced-motion request collapses the period, not the change.
 *
 *  The control that folds it is NOT here. It is one button beside the brand mark, in the same
 *  place at either width (owner decision 2026-09-22): a control that moves when it is pressed
 *  asks to be found again, and the rail holds destinations and nothing else.
 *
 *  `children` is given the current state because a rail draws itself differently at each width —
 *  names beside glyphs when open, a centred column of glyphs when folded. */
export function CollapsibleRail({
  toggle,
  width,
  collapsedWidth = '3.75rem',
  className,
  boxClassName,
  collapsedBoxClassName,
  children,
}: {
  toggle: RailToggle
  /** The column while the rail shows its names, as CSS. */
  width: string
  /** The column while it shows only glyphs: one 44px target and its gutters. */
  collapsedWidth?: string
  /** The aside itself: its plane, its sticky box, and the `lg:` gate that decides it is drawn. */
  className?: string
  /** The open column's own gutters. Keep the `lg:` prefix on any scrolling — the page is the one
   *  scroller at every width below the rail's own. */
  boxClassName?: string
  /** The folded column's, which carries the same vertical rhythm so the control does not step up
   *  or down as the rail folds. Its horizontal gutter is the glyph's own centring. */
  collapsedBoxClassName?: string
  children: (collapsed: boolean) => ReactNode
}) {
  const reduced = useReducedMotion()
  const current = toggle.open ? width : collapsedWidth
  return (
    <motion.aside
      initial={false}
      animate={{ width: current }}
      transition={{ duration: reduced ? 0 : 0.2, ease: [0.2, 0, 0, 1] }}
      // Clipped only while it is showing its names: the open column is wider than the folded one
      // it animates from, so its rows would spill during the change. Folded, nothing inside is
      // wider than the column except a tooltip, which is meant to reach past its edge.
      className={clsx(toggle.open ? 'overflow-hidden' : 'overflow-visible', className)}
      style={{ width: current }}
    >
      {toggle.open ? (
        <div className={clsx('flex flex-col', boxClassName)} style={{ width, minWidth: width }}>
          {children(false)}
        </div>
      ) : (
        <div
          className={clsx('flex flex-col items-center gap-1', collapsedBoxClassName)}
          style={{ width: collapsedWidth, minWidth: collapsedWidth }}
        >
          {children(true)}
        </div>
      )}
    </motion.aside>
  )
}
