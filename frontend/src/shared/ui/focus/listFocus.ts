/** A dropdown's ↑ ↓ Home End over its rows, the programmatic focus stops it passes in DOM order:
 *  the arrows step from the focused row and wrap, Home and End jump to the ends. Those four keys
 *  lose their default even over no rows; every other key is left to the caller. */
export function moveListFocus(
  event: Pick<KeyboardEvent, 'key' | 'preventDefault'>,
  elements: readonly HTMLElement[],
) {
  const { key } = event
  if (key === 'Home' || key === 'End') {
    event.preventDefault()
    elements.at(key === 'Home' ? 0 : -1)?.focus()
    return
  }
  if (key !== 'ArrowDown' && key !== 'ArrowUp') return
  event.preventDefault()
  if (!elements.length) return
  const step = key === 'ArrowDown' ? 1 : -1
  const current = elements.indexOf(document.activeElement as HTMLElement)
  elements[(current + step + elements.length) % elements.length]?.focus()
}
