import { createContext, useContext, type ComponentType, type ReactNode } from 'react'

export interface NavigationDestination {
  href: string
  label: string
}
export interface NavigationLinkProps {
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
