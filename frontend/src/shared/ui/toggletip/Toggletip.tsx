import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Info } from 'lucide-react'
import { Button } from '../button/Button'
import { useAnchoredContentPanel } from '../anchored-panel/useAnchoredContentPanel'

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
  const { triggerRef, disclosure, box, panelProps } = useAnchoredContentPanel<HTMLButtonElement>()
  const { open } = disclosure

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
            {...panelProps}
            aria-hidden="true"
            className="bg-surface-overlay z-overlay-panel max-w-popover text-content-primary fixed w-72 rounded-lg p-4 text-sm shadow-lg"
          >
            {children}
          </div>,
          document.body,
        )}
    </>
  )
}
