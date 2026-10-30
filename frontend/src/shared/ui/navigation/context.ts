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
/** Router links and autosave/unsaved guards are supplied from above shared. */
export interface NavigationValue {
  current: string
  ancestors: readonly NavigationDestination[]
  returnTo?: NavigationDestination
  Link?: ComponentType<NavigationLinkProps>
}
export const NavigationContext = createContext<NavigationValue | null>(null)
export function useNavigationContext() {
  return useContext(NavigationContext)
}
