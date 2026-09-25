/** Everything Tab can actually reach. `[tabindex="-1"]` is deliberately excluded: a listbox's
 *  options are programmatic focus targets, and counting one as the panel's last control lets Tab
 *  escape the modal entirely. */
export const FOCUSABLE_SELECTOR =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

/** The controls Tab reaches inside `root`, in document order; none for no root. */
export function focusablesIn(root: ParentNode | null | undefined): HTMLElement[] {
  return root ? [...root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)] : []
}
