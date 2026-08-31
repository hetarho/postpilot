import { createContext, createElement, useContext, type ReactNode } from 'react'
import type { EffectiveTheme, ThemePreference } from '@/shared/lib'

/** The feature-facing theme port. The app provider owns browser lifecycle and injects only the
 * state and action the selector needs, keeping this feature independent from the app layer. */
export interface ThemeController {
  preference: ThemePreference
  effectiveTheme: EffectiveTheme
  setPreference: (preference: ThemePreference) => void
}

const ThemeControllerContext = createContext<ThemeController | undefined>(undefined)

export function ThemeControllerProvider({
  value,
  children,
}: {
  value: ThemeController
  children: ReactNode
}) {
  return createElement(ThemeControllerContext.Provider, { value }, children)
}

export function useThemeController(): ThemeController {
  const controller = useContext(ThemeControllerContext)
  if (!controller) throw new Error('ThemeControllerProvider is missing')
  return controller
}
