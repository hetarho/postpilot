/** The open listbox panel's placement tuning, beside the control that measures with
 *  it (ARCH-21). Nothing outside this control reads them. */

/** The clear space an open listbox panel keeps from the top or bottom edge of the viewport. It
 *  matches the popover's gutter for the same reason: a bounded overlay should stop where the
 *  content column stops rather than run to the glass. */
export const LISTBOX_VIEWPORT_GUTTER_PX = 16

/** The `mt-1` an open listbox panel keeps between itself and its trigger, as a number, so the
 *  height measurement can subtract the gap the CSS is about to add. */
export const LISTBOX_TRIGGER_GAP_PX = 4

/** The shortest an open listbox panel is squeezed to before it stops honouring the room it
 *  measured — roughly three option rows. A trigger closer than this to BOTH edges is a layout
 *  bug at the call site, and overhanging is the better failure there. */
export const LISTBOX_MIN_PANEL_PX = 132

/** How much of the viewport an open listbox may take when there is more room than it needs. A
 *  forty-model catalog would otherwise open a panel as tall as the screen from a field near the
 *  top of a desktop page, which buries the field it belongs to. */
export const LISTBOX_MAX_VIEWPORT_RATIO = 0.5
