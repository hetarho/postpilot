import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
  type RefObject,
} from 'react'
import { ANCHORED_PANEL_HOVER_CLOSE_MS } from './config'
import { FINE_HOVER_MEDIA_QUERY, useMediaQuery } from '../media-query/useMediaQuery'

/** How a panel is open: not at all, by a press (click, tap, Enter or Space), or by a fine
 *  pointer resting on its trigger. */
export type AnchoredOpen = '' | 'press' | 'hover'

/** The open state of a panel that a press opens under every pointer and a fine pointer may
 *  also open by hovering (THEME-32: nothing is reachable only by hovering).
 *
 *  A press opens a closed panel, pins a hover-opened one, and closes a pressed one. A hover-opened
 *  panel closes a moment after the pointer leaves both the trigger and the panel. An outside press
 *  closes either kind, returning focus to the trigger only when it was opened by a press. */
export function useAnchoredDisclosure(
  triggerRef: RefObject<HTMLElement | null>,
  panelRef: RefObject<HTMLElement | null>,
) {
  const [open, setOpen] = useState<AnchoredOpen>('')
  const fineHover = useMediaQuery(FINE_HOVER_MEDIA_QUERY)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const cancelClose = useCallback(() => {
    clearTimeout(timer.current)
    timer.current = undefined
  }, [])

  const close = useCallback(
    (returnFocus: boolean, preventScroll = false) => {
      cancelClose()
      setOpen('')
      if (returnFocus) queueMicrotask(() => triggerRef.current?.focus({ preventScroll }))
    },
    [cancelClose, triggerRef],
  )

  const press = () => {
    cancelClose()
    setOpen((current) => (current === 'press' ? '' : 'press'))
  }

  /** A press on a hover-opened panel keeps it: the user reached into it. */
  const pin = () => {
    cancelClose()
    setOpen((current) => (current === 'hover' ? 'press' : current))
  }

  const pointerEnter = (event: PointerEvent) => {
    cancelClose()
    // A tap's emulated enter reports `touch` or `pen`, so it never opens a panel it did not press.
    if (fineHover && event.pointerType === 'mouse') setOpen((current) => current || 'hover')
  }

  const pointerLeave = () => {
    if (open !== 'hover') return
    cancelClose()
    timer.current = setTimeout(() => {
      timer.current = undefined
      setOpen((current) => (current === 'hover' ? '' : current))
    }, ANCHORED_PANEL_HOVER_CLOSE_MS)
  }

  /** Escape closes this panel and nothing around it. The panel may be open INSIDE an overlay that
   *  also closes on Escape (a popover, a sheet), and one Escape must dismiss one thing — the
   *  innermost. React delivers portalled events through the tree, and stopping the event here
   *  stops the native one at the portal or root container, before it reaches the document
   *  listeners those overlays install (THEME-30). A closed panel lets Escape through, so it still
   *  closes the container.
   *
   *  A HOVER-opened panel has no focus inside it, so no key event passes through its tree to be
   *  stopped here; the effect below catches Escape for it on the document instead. */
  const escape = (event: KeyboardEvent) => {
    if (event.key !== 'Escape' || !open) return
    event.preventDefault()
    event.stopPropagation()
    close(true)
  }

  useEffect(() => {
    if (!open) return
    const onPointerDown = (event: globalThis.PointerEvent) => {
      const target = event.target as Node | null
      // The panel is portalled out of the trigger's tree, so both have to be named as inside.
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) return
      close(open === 'press')
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [close, open, panelRef, triggerRef])

  // Escape for a panel a fine pointer opened by resting on its trigger (THEME-32). A rest moves no
  // focus, so the key's target is whatever held focus before — a control of a popover the panel
  // may sit in. A CAPTURE listener on the document runs before that popover's and a sheet's
  // document listeners and before React's root, so stopping it here dismisses one thing: this
  // panel. That is THEME-30's re-check for a capture listener; a press-opened panel keeps the
  // element-level `escape` above.
  useEffect(() => {
    if (open !== 'hover') return
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      close(panelRef.current?.contains(document.activeElement) ?? false)
    }
    document.addEventListener('keydown', onKeyDown, true)
    return () => document.removeEventListener('keydown', onKeyDown, true)
  }, [close, open, panelRef])

  useEffect(() => cancelClose, [cancelClose])

  return { open, close, press, pin, pointerEnter, pointerLeave, escape }
}
