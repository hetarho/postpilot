import { forwardRef, useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { twMerge } from 'tailwind-merge'
import { Button, type ButtonProps } from '../button/Button'
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

interface ExplainedButtonProps extends ButtonProps {
  /** Why a disabled button is off, as one short sentence. It is shown as a tooltip on the button
   *  itself, so no line under the button has to say it. '' — or an enabled or pending button —
   *  is an ordinary `Button`. */
  reason?: string
  /** On the wrapper, which is the box a layout places: a grid cell or a flex item. */
  wrapperClassName?: string
}

/** A `Button` that, while it is off for a reason worth stating, says that reason in a tooltip
 *  instead of in a line of copy beside it: under a resting fine pointer, and after a press under
 *  every pointer and from the keyboard (THEME-32: nothing is reachable only by hovering).
 *
 *  A natively `disabled` button takes no focus and no pointer events, so nothing could reach its
 *  reason. While it explains, it is `aria-disabled` instead: still focusable, announced as
 *  unavailable, its reason as its description, and its own `onClick` never runs. `buttonStyles`
 *  gives an `aria-disabled` button no pointer events, so a press or a rest lands on the WRAPPER,
 *  which is also what the bubble is anchored to; a keyboard press is a click on the button that
 *  bubbles to the same handler. The bubble is portalled like `Toggletip`'s (THEME-30) and is
 *  `aria-hidden`, since the description already carries its text. */
export const ExplainedButton = forwardRef<HTMLButtonElement, ExplainedButtonProps>(
  function ExplainedButton(
    {
      reason = '',
      wrapperClassName,
      disabled = false,
      pending = false,
      onClick,
      'aria-describedby': describedBy,
      ...props
    },
    ref,
  ) {
    const reasonId = useId()
    const wrapperRef = useRef<HTMLSpanElement>(null)
    const panelRef = useRef<HTMLDivElement>(null)
    const disclosure = useAnchoredDisclosure(wrapperRef, panelRef)
    const { open, close } = disclosure
    const explaining = Boolean(reason) && disabled && !pending
    const box = useAnchoredPanel({
      open: explaining && open !== '',
      triggerRef: wrapperRef,
      panelRef,
      placement: PLACEMENT,
      width: 'content',
      onGone: (focusWasInside) => close(focusWasInside, true),
    })

    // A button that stops explaining — enabled, or off for a reason it does not state — takes its
    // tip with it, so the tip cannot reappear on its own the next time the reason comes back.
    useEffect(() => {
      if (!explaining) close(false)
    }, [close, explaining])

    return (
      // `grid`, so the button fills whatever box the layout gives the wrapper — a 3 : 7 cell on a
      // phone — and keeps its natural width where the wrapper has one.
      <span
        ref={wrapperRef}
        className={twMerge('grid', wrapperClassName)}
        onClick={explaining ? disclosure.press : undefined}
        onKeyDown={explaining ? disclosure.escape : undefined}
        onPointerEnter={explaining ? disclosure.pointerEnter : undefined}
        onPointerLeave={explaining ? disclosure.pointerLeave : undefined}
      >
        <Button
          ref={ref}
          {...props}
          disabled={disabled && !explaining}
          pending={pending}
          aria-disabled={explaining || undefined}
          aria-describedby={
            explaining ? [describedBy, reasonId].filter(Boolean).join(' ') : describedBy
          }
          onClick={explaining ? undefined : onClick}
        />
        {explaining && (
          <span id={reasonId} className="sr-only">
            {reason}
          </span>
        )}
        {explaining &&
          open &&
          box &&
          createPortal(
            <div
              ref={panelRef}
              aria-hidden="true"
              {...{ [ANCHORED_PANEL_ATTRIBUTE]: '' }}
              onPointerDown={disclosure.pin}
              onPointerEnter={disclosure.pointerEnter}
              onPointerLeave={disclosure.pointerLeave}
              className="bg-surface-overlay z-overlay-panel max-w-popover text-content-primary fixed w-max rounded-md px-3 py-2 text-sm shadow-lg"
              style={{ left: box.left, top: box.top, bottom: box.bottom }}
            >
              {reason}
            </div>,
            document.body,
          )}
      </span>
    )
  },
)
