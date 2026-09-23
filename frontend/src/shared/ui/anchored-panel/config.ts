/** Placement and hover tuning for the anchored panels `InlinePopover` and `Toggletip` open,
 *  beside the hook that measures with them (ARCH-21). `Listbox` keeps its own numbers in
 *  `listbox/config.ts`. */

/** The clear space a panel keeps from the viewport's edges — the page's own `px-4`, so a
 *  corrected panel lines up with the content column instead of the glass. */
export const ANCHORED_PANEL_VIEWPORT_GUTTER_PX = 16

/** The gap between the trigger and the panel, as a number, so the room measurement can subtract
 *  it before the panel is placed. */
export const ANCHORED_PANEL_TRIGGER_GAP_PX = 4

/** The shortest a panel is squeezed to before it stops honouring the room it measured. */
export const ANCHORED_PANEL_MIN_PX = 120

/** How much of the viewport a panel may take when there is more room than it needs. */
export const ANCHORED_PANEL_MAX_VIEWPORT_RATIO = 0.5

/** How long a hover-opened panel waits after the pointer leaves before it closes, so the pointer
 *  can cross the gap from the trigger to the panel. */
export const ANCHORED_PANEL_HOVER_CLOSE_MS = 150
