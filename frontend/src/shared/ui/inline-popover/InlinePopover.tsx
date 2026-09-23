import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { twMerge } from 'tailwind-merge'
import {
  ANCHORED_PANEL_MAX_VIEWPORT_RATIO,
  ANCHORED_PANEL_MIN_PX,
  ANCHORED_PANEL_TRIGGER_GAP_PX,
  ANCHORED_PANEL_VIEWPORT_GUTTER_PX,
} from '../anchored-panel/config'
import { useAnchoredDisclosure } from '../anchored-panel/useAnchoredDisclosure'
import { ANCHORED_PANEL_ATTRIBUTE, useAnchoredPanel } from '../anchored-panel/useAnchoredPanel'

const PLACEMENT = {
  gapPx: ANCHORED_PANEL_TRIGGER_GAP_PX,
  gutterPx: ANCHORED_PANEL_VIEWPORT_GUTTER_PX,
  minPx: ANCHORED_PANEL_MIN_PX,
  maxViewportRatio: ANCHORED_PANEL_MAX_VIEWPORT_RATIO,
}

const focusableSelector =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

interface InlinePopoverProps {
  /** Names the panel. The trigger's own name is its text. */
  label: string
  /** The trigger's text, which reads as part of the sentence around it. */
  children: ReactNode
  panel: (close: () => void) => ReactNode
  /** On the trigger. */
  className?: string
  disabled?: boolean
}

/** A press-operable panel opened from a mark INSIDE prose: a phrase in a sentence that holds more
 *  than the sentence can show. A click, Enter or Space opens it under every pointer; a fine
 *  pointer may also open it by resting on the mark, which never moves focus (THEME-32).
 *
 *  The trigger is a native button laid out inline, so it wraps with the line it sits in. Its size
 *  is set by the line-height of the prose around it, which is WCAG 2.5.8's inline exception: it
 *  does not grow to THEME-23's 44 px under a coarse pointer, unlike an icon button such as
 *  `Toggletip`, which keeps the pointer floor.
 *
 *  The panel is portalled and placed by `useAnchoredPanel` (THEME-30). A press or key opening
 *  moves focus into it; Escape closes only it and returns focus to the trigger; Tab hands focus
 *  back to the trigger so traversal continues in the flow; an outside press closes it; and it is
 *  given up once the trigger scrolls out of the viewport. */
export function InlinePopover({
  label,
  children,
  panel,
  className,
  disabled = false,
}: InlinePopoverProps) {
  const panelId = `${useId()}-panel`
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  // Set by a press that opens or pins the panel, and spent by the first render that mounts it.
  const focusIn = useRef(false)
  const disclosure = useAnchoredDisclosure(triggerRef, panelRef)
  const { open, close } = disclosure
  const box = useAnchoredPanel({
    open: open !== '',
    triggerRef,
    panelRef,
    placement: PLACEMENT,
    width: 'content',
    onGone: (focusWasInside) => close(focusWasInside, true),
  })

  useEffect(() => {
    if (!open || !box || !focusIn.current) return
    focusIn.current = false
    queueMicrotask(() => {
      const first = panelRef.current?.querySelector<HTMLElement>(focusableSelector)
      ;(first ?? panelRef.current)?.focus()
    })
  }, [open, box])

  const onPanelKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Tab') {
      // The panel lives at the end of the document, so leaving focus in it would send Tab off the
      // end of the page. From the trigger the native traversal continues in the flow, and any
      // focus trap the trigger sits inside (a sheet's) sees a target it still owns.
      triggerRef.current?.focus()
      close(false)
      return
    }
    disclosure.escape(event)
  }

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        disabled={disabled}
        aria-haspopup="dialog"
        aria-expanded={open !== ''}
        aria-controls={open ? panelId : undefined}
        onClick={() => {
          focusIn.current = open !== 'press'
          disclosure.press()
        }}
        onKeyDown={disclosure.escape}
        onPointerEnter={disclosure.pointerEnter}
        onPointerLeave={disclosure.pointerLeave}
        className={twMerge(
          'hover:bg-row-bg-hover active:bg-row-bg-active inline rounded-sm text-left break-words whitespace-normal underline decoration-dotted underline-offset-4',
          className,
        )}
      >
        {children}
      </button>
      {open &&
        box &&
        createPortal(
          <div
            ref={panelRef}
            id={panelId}
            role="dialog"
            aria-label={label}
            tabIndex={-1}
            {...{ [ANCHORED_PANEL_ATTRIBUTE]: '' }}
            onKeyDown={onPanelKeyDown}
            onPointerDown={disclosure.pin}
            onPointerEnter={disclosure.pointerEnter}
            onPointerLeave={disclosure.pointerLeave}
            // `bg-surface-overlay`: the panel can open over a `Sheet`, whose own surface is
            // `surface-highest`, exactly as a listbox's can. `rounded-lg p-4` never shares the
            // trigger's step (THEME-23).
            className="bg-surface-overlay z-overlay-panel max-w-popover fixed w-72 overflow-y-auto overscroll-contain rounded-lg p-4 shadow-lg"
            style={{ left: box.left, top: box.top, bottom: box.bottom, maxHeight: box.maxHeight }}
          >
            {panel(() => close(true))}
          </div>,
          document.body,
        )}
    </>
  )
}
