/** The popover panel's placement tuning, beside the control that measures with it
 *  (ARCH-21). Nothing outside this control reads them. */

/** The clear space a popover panel keeps from either viewport edge when its anchor would push it
 *  past one. It matches the page's own `px-4` gutter, so a corrected panel lines up with the
 *  content column instead of floating against the glass. */
export const POPOVER_VIEWPORT_GUTTER_PX = 16

/** The `mt-2` / `mb-2` a popover panel keeps between itself and its trigger, as a number, so the
 *  height measurement can subtract the gap the CSS is about to add. */
export const POPOVER_TRIGGER_GAP_PX = 8

/** The shortest a popover panel is squeezed to before it stops honouring the room it measured.
 *  Below roughly three rows a scroller is worse than a panel that overhangs the viewport edge a
 *  little, and a trigger that close to the edge is a layout bug to fix at the call site. */
export const POPOVER_MIN_PANEL_PX = 160
