import { useCallback, useSyncExternalStore } from 'react'

/** Tailwind's `sm:` breakpoint, as a media query.
 *
 *  A responsive shape is normally a CSS concern, and stays one. This exists for the one case where
 *  the two shapes cannot both be MOUNTED: a bottom sheet locks the body scroll and traps focus
 *  from the moment it renders, so hiding it with `sm:hidden` beside a popover would leave the wide
 *  screen unable to scroll. `Popover` therefore has to pick one, and it picks here.
 *
 *  The hook lives in `shared/ui` rather than `shared/lib`, which is a react-free layer by
 *  construction (the ESLint boundaries rule), and in its own directory since a second primitive
 *  needed it — note that a dialog-or-sheet surface is NOT one: `Sheet` already is a bottom sheet
 *  on a phone and a centred dialog from `md:` up, in CSS, on one mount. */
export const SM_MEDIA_QUERY = '(min-width: 40rem)'

/** A pointer that can hover and aim finely: a mouse or a trackpad. Only under it does hovering
 *  open an anchored panel — on touch an emulated enter would open one a tap never asked for, and
 *  nothing may be reachable only by hovering (THEME-32). */
export const FINE_HOVER_MEDIA_QUERY = '(hover: hover) and (pointer: fine)'

/** Subscribes to a media query, re-rendering when it starts or stops matching. Reports `false`
 *  wherever `matchMedia` is absent, so a component's phone shape is the fallback — the base
 *  breakpoint IS the design (design-language §1.5). */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onStoreChange: () => void) => {
      const list = window.matchMedia?.(query)
      if (!list) return () => undefined
      list.addEventListener('change', onStoreChange)
      return () => list.removeEventListener('change', onStoreChange)
    },
    [query],
  )
  const getSnapshot = useCallback(() => window.matchMedia?.(query).matches ?? false, [query])
  return useSyncExternalStore(subscribe, getSnapshot)
}
