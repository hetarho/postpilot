import type { ReactNode } from 'react'
import { NavigationContext, type NavigationValue } from './context'

export function NavigationProvider({
  value,
  children,
}: {
  value: NavigationValue
  children: ReactNode
}) {
  return <NavigationContext.Provider value={value}>{children}</NavigationContext.Provider>
}
