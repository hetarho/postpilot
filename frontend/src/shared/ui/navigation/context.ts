import {
  createContext,
  useContext,
  type AnchorHTMLAttributes,
  type ComponentType,
  type ReactNode,
} from 'react'

export interface NavigationDestination {
  href: string
  label: string
}
/** An anchor's own attributes pass through, so a menu row can carry its role and focus order;
 *  a supplied `onClick` runs before the router takes the click. */
export interface NavigationLinkProps extends Omit<
  AnchorHTMLAttributes<HTMLAnchorElement>,
  'href' | 'children' | 'className'
> {
  href: string
  children: ReactNode
  className?: string
}
/** Router links and autosave/unsaved guards are supplied from above shared. The route's
 *  structural ancestors, nearest last, and a validated contextual return decide where its one
 *  back link leads (THEME-58). */
export interface NavigationValue {
  ancestors: readonly NavigationDestination[]
  returnTo?: NavigationDestination
  Link?: ComponentType<NavigationLinkProps>
  /** Whether this route has a way back at all (a child route outside a focused flow). */
  hasReturn?: boolean
  /** A page that draws the way back in its own column (`PageFrame`) claims it, so the shell does
   *  not draw a second one; the returned function releases the claim on unmount. */
  claimReturn?: () => () => void
}
export const NavigationContext = createContext<NavigationValue | null>(null)
export function useNavigationContext() {
  return useContext(NavigationContext)
}
