import type { EffectiveTheme } from '@/shared/lib'

type ColorScheme = 'light' | 'dark'

const COLOR_SCHEME = {
  day: 'light',
  night: 'dark',
} as const satisfies Record<EffectiveTheme, ColorScheme>

function browserDocument(): Document | undefined {
  try {
    return typeof document === 'undefined' ? undefined : document
  } catch {
    return undefined
  }
}

function meta(target: Document, name: string): HTMLMetaElement {
  const existing = target.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (existing) return existing

  const created = target.createElement('meta')
  created.name = name
  target.head.append(created)
  return created
}

/** Keeps the semantic theme, native controls, and browser chrome on one effective-theme value. */
export function applyDocumentTheme(
  effectiveTheme: EffectiveTheme,
  target: Document | null | undefined = browserDocument(),
): void {
  if (!target) return

  const colorScheme = COLOR_SCHEME[effectiveTheme]
  const themeColor = meta(target, 'theme-color')

  target.documentElement.dataset.theme = effectiveTheme
  target.documentElement.style.colorScheme = colorScheme
  meta(target, 'color-scheme').content = colorScheme
  themeColor.content = themeColor.dataset[effectiveTheme] ?? ''
}
