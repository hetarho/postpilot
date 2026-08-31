import { DEFAULT_THEME_PREFERENCE } from '@/shared/config'

export const THEME_PREFERENCES = ['system', 'light', 'dark'] as const
export const EFFECTIVE_THEMES = ['day', 'night'] as const

export type ThemePreference = (typeof THEME_PREFERENCES)[number]
export type StoredThemePreference = Exclude<ThemePreference, typeof DEFAULT_THEME_PREFERENCE>
export type EffectiveTheme = (typeof EFFECTIVE_THEMES)[number]

export function isThemePreference(value: unknown): value is ThemePreference {
  return typeof value === 'string' && THEME_PREFERENCES.includes(value as ThemePreference)
}

export function isEffectiveTheme(value: unknown): value is EffectiveTheme {
  return typeof value === 'string' && EFFECTIVE_THEMES.includes(value as EffectiveTheme)
}

/** System is represented by an absent key, so only exact explicit overrides are stored. */
export function parseStoredThemePreference(value: unknown): StoredThemePreference | undefined {
  return value === 'light' || value === 'dark' ? value : undefined
}

export function resolveEffectiveTheme(
  preference: ThemePreference,
  prefersDark: boolean,
): EffectiveTheme {
  if (preference === 'dark') return 'night'
  if (preference === 'light') return 'day'
  return prefersDark ? 'night' : 'day'
}
