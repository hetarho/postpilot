import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { twMerge } from 'tailwind-merge'
import { useAnchoredContentPanel } from '../anchored-panel/useAnchoredContentPanel'
import { focusablesIn } from '../focus/focusable'

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
 *  moves focus into it; Escape closes only it and returns focus to the trigger; an outside press
 *  closes it; and it is given up once the trigger scrolls out of the viewport. Tab walks every
 *  control in the panel and leaves it past either end (THEME-42): past the last onto the page
 *  control after the trigger, past the first onto the trigger itself. */
export function InlinePopover({
  label,
  children,
  panel,
  className,
  disabled = false,
}: InlinePopoverProps) {
  const panelId = `${useId()}-panel`
  const { triggerRef, panelRef, disclosure, box, panelProps } =
    useAnchoredContentPanel<HTMLButtonElement>()
  const { open, close } = disclosure
  // Set by a press that opens or pins the panel, and spent by the first render that mounts it.
  const focusIn = useRef(false)

  useEffect(() => {
    if (!open || !box || !focusIn.current) return
    focusIn.current = false
    queueMicrotask(() => (focusablesIn(panelRef.current)[0] ?? panelRef.current)?.focus())
  }, [open, box, panelRef])

  const onPanelKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'Tab') {
      disclosure.escape(event)
      return
    }
    // A composite moved by arrows counts as ONE control: a `Listbox` in the panel handles its own
    // Tab first and puts focus back on its trigger, so `document.activeElement` (not the event's
    // target) is what says where the traversal stands.
    const controls = focusablesIn(panelRef.current)
    const active = document.activeElement
    if (event.shiftKey) {
      if (controls.length === 0 || active === controls[0] || active === panelRef.current) {
        // Onto the trigger, not past it: a disclosure's content follows its button in reading
        // order, so the native Shift+Tab from the trigger would skip the mark the user opened.
        // Stopped, or a trap around the trigger would take Shift+Tab on its first control as a
        // wrap to its last.
        event.preventDefault()
        event.stopPropagation()
        triggerRef.current?.focus()
        close(false)
        return
      }
    } else if (controls.length === 0 || active === controls.at(-1)) {
      // The panel lives at the end of the document, so leaving focus in it would send Tab off the
      // end of the page. From the trigger the native traversal continues in the flow, and any
      // focus trap the trigger sits inside (a sheet's, a popover's) sees a target it still owns.
      triggerRef.current?.focus()
      close(false)
      return
    }
    // Between two of the panel's own controls. The panel is portalled to the end of the body, so
    // the native Tab reaches its neighbour; stopping the event at the portal keeps a surrounding
    // trap, which counts focus outside its own panel as a reason to wrap, from moving it.
    event.stopPropagation()
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
            {...panelProps}
            id={panelId}
            role="dialog"
            aria-label={label}
            tabIndex={-1}
            onKeyDown={onPanelKeyDown}
            // `bg-surface-overlay`: the panel can open over a `Sheet`, whose own surface is
            // `surface-highest`, exactly as a listbox's can. `rounded-lg p-4` never shares the
            // trigger's step (THEME-23).
            className="bg-surface-overlay z-overlay-panel max-w-popover fixed w-72 overflow-y-auto overscroll-contain rounded-lg p-4 shadow-lg"
            style={{ ...panelProps.style, maxHeight: box.maxHeight }}
          >
            {panel(() => close(true))}
          </div>,
          document.body,
        )}
    </>
  )
}
