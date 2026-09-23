import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Info } from 'lucide-react'
import { Button } from '../button/Button'
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

interface ToggletipProps {
  /** The button's name: what the tip explains. */
  label: string
  /** Plain text only. A tip with anything to press in it is an `InlinePopover`. */
  children: ReactNode
  className?: string
}

/** An icon button that opens a short explanation beside the control it explains — a press under
 *  every pointer, a rest under a fine pointer (THEME-32). Focus stays on the button: the bubble
 *  holds text alone, so there is nothing to move into, and Escape on the button closes only the
 *  tip.
 *
 *  The bubble is portalled and placed by `useAnchoredPanel` (THEME-30) and is `aria-hidden`; what
 *  it says is mirrored into a `status` region beside the button, which is how the text is
 *  announced on open. The region is always mounted and filled only while the tip is open,
 *  because a region inserted with its text already inside announces nothing.
 *
 *  Put it BESIDE a checkbox's label, never inside it: a press inside a `<label>` would also toggle
 *  the box it labels. */
export function Toggletip({ label, children, className }: ToggletipProps) {
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
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

  return (
    <>
      <Button
        ref={triggerRef}
        variant="ghost"
        size="icon"
        aria-label={label}
        aria-expanded={open !== ''}
        className={className}
        onClick={disclosure.press}
        onKeyDown={disclosure.escape}
        onPointerEnter={disclosure.pointerEnter}
        onPointerLeave={disclosure.pointerLeave}
      >
        <Info aria-hidden="true" className="size-5" />
      </Button>
      <span role="status" className="sr-only">
        {open ? children : null}
      </span>
      {open &&
        box &&
        createPortal(
          <div
            ref={panelRef}
            aria-hidden="true"
            {...{ [ANCHORED_PANEL_ATTRIBUTE]: '' }}
            onPointerDown={disclosure.pin}
            onPointerEnter={disclosure.pointerEnter}
            onPointerLeave={disclosure.pointerLeave}
            className="bg-surface-overlay z-overlay-panel max-w-popover text-content-primary fixed w-72 rounded-lg p-4 text-sm shadow-lg"
            style={{ left: box.left, top: box.top, bottom: box.bottom }}
          >
            {children}
          </div>,
          document.body,
        )}
    </>
  )
}
