import type { ParsedLocation } from '@tanstack/react-router'

/** Where the router keeps a screen's scroll position. Every other screen keeps the router's
 *  default — its own history entry — so going back lands where that entry was left and a new
 *  navigation starts at the top. `/posts` is keyed by its address instead: the editor's 목록 is a
 *  new navigation, not a step back, and coming back to the list by either way keeps the rows that
 *  were loaded and where the reader was among them (POST-93). The narrowing is part of the
 *  address, so each narrowing keeps its own place.
 *
 *  A per-route `scrollRestoration` predicate cannot do this: the router skips its reset to the top
 *  too for a location the predicate turns down, so every other screen would open wherever the one
 *  before it had been scrolled to. */
export function scrollRestorationKey(location: ParsedLocation): string {
  if (location.pathname === '/posts') return location.href
  return location.state.__TSR_key || location.href
}
