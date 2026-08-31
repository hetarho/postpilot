import {
  resolveBrowserTheme,
  type BrowserThemeSnapshot,
  type ThemeBrowserPorts,
} from '@/shared/lib'
import { applyDocumentTheme } from './document-theme'

export type ThemeBootstrapSnapshot = BrowserThemeSnapshot

export interface ThemeBootstrapOptions extends ThemeBrowserPorts {
  targetDocument?: Document | null
}

/** Resolves and applies the browser theme synchronously, before React is allowed to render. */
export function bootstrapTheme({
  targetDocument,
  ...browserPorts
}: ThemeBootstrapOptions = {}): ThemeBootstrapSnapshot {
  const snapshot = resolveBrowserTheme(browserPorts)
  applyDocumentTheme(snapshot.effectiveTheme, targetDocument)
  return Object.freeze(snapshot)
}
