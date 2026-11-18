import { useLayoutEffect } from 'react'
import { useNavigationContext } from './context'

/** Whether this place draws the route's way back, claimed from whatever would otherwise draw it —
 *  the shell above the page, or the page's frame — so the route still shows exactly one
 *  (THEME-58). `enabled: false` draws none and claims nothing. */
export function useReturnClaim(enabled = true): boolean {
  const navigation = useNavigationContext()
  const claim = navigation?.claimReturn
  const show = enabled && !!navigation?.hasReturn
  // Before paint, so the outer return never flashes beside this one.
  useLayoutEffect(() => (show ? claim?.() : undefined), [claim, show])
  return show
}
