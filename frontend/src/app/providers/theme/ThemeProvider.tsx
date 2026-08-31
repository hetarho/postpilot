import { useCallback, useLayoutEffect, useMemo, useState, type ReactNode } from 'react'
import { ThemeControllerProvider, type ThemeController } from '@/features/change-theme'
import { DEFAULT_THEME_PREFERENCE, THEME_PREFERENCE_STORAGE_KEY } from '@/shared/config'
import {
  browserThemeMediaQuery,
  browserThemeStorage,
  parseStoredThemePreference,
  readPrefersDark,
  readThemePreference,
  resolveEffectiveTheme,
  writeThemePreference,
  type ThemeMediaQuery,
  type ThemePreference,
  type ThemeStorage,
} from '@/shared/lib'
import type { ThemeBootstrapSnapshot } from './bootstrap'
import { applyDocumentTheme } from './document-theme'

export interface ThemeStorageEventTarget {
  addEventListener(type: 'storage', listener: (event: StorageEvent) => void): void
  removeEventListener(type: 'storage', listener: (event: StorageEvent) => void): void
}

export interface ThemeRuntimePorts {
  storage?: ThemeStorage | null
  mediaQuery?: ThemeMediaQuery | null
  storageEvents?: ThemeStorageEventTarget | null
  targetDocument?: Document | null
}

interface ThemeRuntimeState {
  preference: ThemePreference
  prefersDark: boolean
}

interface ResolvedThemeRuntimePorts {
  storage: ThemeStorage | null
  mediaQuery: ThemeMediaQuery | null
  storageEvents: ThemeStorageEventTarget | null
  targetDocument: Document | null
}

function browserStorageEvents(): ThemeStorageEventTarget | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window
  } catch {
    return undefined
  }
}

function browserDocument(): Document | undefined {
  try {
    return typeof document === 'undefined' ? undefined : document
  } catch {
    return undefined
  }
}

function runtimePorts(
  snapshot: ThemeBootstrapSnapshot,
  supplied: ThemeRuntimePorts,
): ResolvedThemeRuntimePorts {
  return {
    storage: supplied.storage === null ? null : (supplied.storage ?? browserThemeStorage() ?? null),
    mediaQuery:
      supplied.mediaQuery === null
        ? null
        : (supplied.mediaQuery ?? snapshot.mediaQuery ?? browserThemeMediaQuery() ?? null),
    storageEvents:
      supplied.storageEvents === null
        ? null
        : (supplied.storageEvents ?? browserStorageEvents() ?? null),
    targetDocument:
      supplied.targetDocument === null
        ? null
        : (supplied.targetDocument ?? browserDocument() ?? null),
  }
}

export function ThemeProvider({
  initialSnapshot,
  ports = {},
  children,
}: {
  initialSnapshot: ThemeBootstrapSnapshot
  ports?: ThemeRuntimePorts
  children: ReactNode
}) {
  const [runtime] = useState(() => runtimePorts(initialSnapshot, ports))
  const [state, setState] = useState<ThemeRuntimeState>(() => ({
    preference: initialSnapshot.preference,
    prefersDark: initialSnapshot.prefersDark,
  }))

  const effectiveTheme = resolveEffectiveTheme(state.preference, state.prefersDark)

  useLayoutEffect(() => {
    applyDocumentTheme(effectiveTheme, runtime.targetDocument)
  }, [effectiveTheme, runtime.targetDocument])

  useLayoutEffect(() => {
    const mediaQuery = runtime.mediaQuery
    if (state.preference !== DEFAULT_THEME_PREFERENCE || !mediaQuery) return

    const onChange = () => {
      const prefersDark = readPrefersDark(mediaQuery)
      setState((current) =>
        current.preference === DEFAULT_THEME_PREFERENCE && current.prefersDark !== prefersDark
          ? { ...current, prefersDark }
          : current,
      )
    }

    let listening = false
    try {
      if (mediaQuery.addEventListener && mediaQuery.removeEventListener) {
        mediaQuery.addEventListener('change', onChange)
        listening = true
      }
    } catch {
      // A readable media query still provides the latest snapshot even when events are blocked.
    }
    onChange()
    if (!listening) return

    return () => {
      try {
        mediaQuery.removeEventListener?.('change', onChange)
      } catch {
        // Listener cleanup is best-effort for browser ports that became unavailable.
      }
    }
  }, [runtime.mediaQuery, state.preference])

  useLayoutEffect(() => {
    const events = runtime.storageEvents

    const reconcilePreference = (preference: ThemePreference) => {
      setState((current) => {
        const prefersDark =
          preference === DEFAULT_THEME_PREFERENCE
            ? readPrefersDark(runtime.mediaQuery)
            : current.prefersDark
        return current.preference === preference && current.prefersDark === prefersDark
          ? current
          : { preference, prefersDark }
      })
    }
    const onStorage = (event: StorageEvent) => {
      if (event.key !== THEME_PREFERENCE_STORAGE_KEY && event.key !== null) return
      if (event.storageArea && event.storageArea !== runtime.storage) return

      reconcilePreference(parseStoredThemePreference(event.newValue) ?? DEFAULT_THEME_PREFERENCE)
    }

    let listening = false
    try {
      events?.addEventListener('storage', onStorage)
      listening = events !== null
    } catch {
      // Reconciliation below remains safe when storage events are blocked.
    }
    if (runtime.storage) reconcilePreference(readThemePreference(runtime.storage))
    if (!listening) return

    return () => {
      try {
        events?.removeEventListener('storage', onStorage)
      } catch {
        // Listener cleanup is best-effort for browser ports that became unavailable.
      }
    }
  }, [runtime.mediaQuery, runtime.storage, runtime.storageEvents])

  const setPreference = useCallback(
    (preference: ThemePreference) => {
      setState((current) => ({
        preference,
        prefersDark:
          preference === DEFAULT_THEME_PREFERENCE
            ? readPrefersDark(runtime.mediaQuery)
            : current.prefersDark,
      }))
      writeThemePreference(preference, runtime.storage)
    },
    [runtime.mediaQuery, runtime.storage],
  )

  const controller = useMemo<ThemeController>(
    () => ({ preference: state.preference, effectiveTheme, setPreference }),
    [effectiveTheme, setPreference, state.preference],
  )

  return <ThemeControllerProvider value={controller}>{children}</ThemeControllerProvider>
}
