import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import {
  POPOVER_VIEWPORT_GUTTER_PX,
  POPOVER_MIN_PANEL_PX,
  POPOVER_TRIGGER_GAP_PX,
} from '@/shared/config'
import { Button } from '../button/Button'
import type { ButtonSize, ButtonVariant } from '../button/buttonStyles'
import { Sheet } from '../sheet/Sheet'
import { SM_MEDIA_QUERY, useMediaQuery } from './useMediaQuery'

const focusableSelector =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

/** Opens the surface from elsewhere on the page. The editor's generation empty state offers a way
 *  into the writing brief it is blocked on, and an imperative handle keeps the popover's own state
 *  where it belongs — the listeners that close it capture one stable `close` for their lifetime. */
export interface PopoverHandle {
  open: () => void
}

export const Popover = forwardRef<
  PopoverHandle,
  {
    label: string
    triggerLabel?: ReactNode
    /** `icon` for a glyph-only trigger. `label` is already the button's `aria-label`, so the
     *  control keeps its name — and the panel keeps its heading — with no visible text. */
    triggerSize?: ButtonSize
    /** The trigger's emphasis. `secondary` unless the surface it opens IS the step's committing
     *  choice — 확정하기 opens the pair that ends 글 다듬기, so it carries the CTA fill. */
    triggerVariant?: ButtonVariant
    triggerClassName?: string
    children: (close: () => void) => ReactNode
    disabled?: boolean
    /** Above remains the action-bar default; compact header controls explicitly open below. */
    placement?: 'above' | 'below'
    /** Which edge of the panel is pinned to the trigger. `end` is the action-bar default: a dock
     *  control's panel grows leftwards, away from the viewport edge it sits nearest. */
    align?: 'start' | 'end'
    /** `sheet` swaps the panel for a full-bleed bottom sheet below `sm:`. A 288px panel opening
     *  over the draft is a desktop shape borrowed by a phone; a long brief needs the whole screen
     *  and its own scroller (design-language §7). */
    phone?: 'popover' | 'sheet'
    /** On the popover's ROOT, so a caller can make the trigger share a flex row. */
    className?: string
  }
>(function Popover(
  {
    label,
    triggerLabel,
    triggerSize,
    triggerVariant = 'secondary',
    triggerClassName,
    children,
    disabled = false,
    placement = 'above',
    align = 'end',
    phone = 'popover',
    className,
  },
  ref,
) {
  const { t } = useTranslation('common')
  const [open, setOpen] = useState(false)
  const id = useId()
  const headingId = `${id}-heading`
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const wide = useMediaQuery(SM_MEDIA_QUERY)
  // The two shapes cannot both be mounted: an open `Sheet` locks the body scroll and traps focus
  // regardless of whether CSS is painting it.
  const asSheet = phone === 'sheet' && !wide

  const close = () => {
    setOpen(false)
  }

  useImperativeHandle(ref, () => ({ open: () => setOpen(true) }), [])

  // The panel is anchored to the trigger, and a trigger can sit anywhere in the viewport. Nothing
  // in CSS knows where, so both bounds are measured here.
  //
  // HEIGHT first: the panel opens into the gap between the trigger and the edge it grows toward,
  // and a `dvh`-based token can only guess at that gap. A tall surface — the whole writing brief —
  // has to scroll inside the gap it actually has, because the part that runs past the viewport
  // edge is unreachable: the page behind is scroll-locked on a phone and the panel is anchored to
  // a docked bar that does not move on a pointer. The floor keeps the panel usable rather than
  // squeezing it to a sliver when the trigger sits almost against that edge.
  useLayoutEffect(() => {
    const panel = panelRef.current
    const trigger = triggerRef.current
    if (!open || asSheet || !panel || !trigger) return
    panel.style.transform = ''
    const anchor = trigger.getBoundingClientRect()
    const room =
      placement === 'above'
        ? anchor.top - POPOVER_TRIGGER_GAP_PX - POPOVER_VIEWPORT_GUTTER_PX
        : window.innerHeight - anchor.bottom - POPOVER_TRIGGER_GAP_PX - POPOVER_VIEWPORT_GUTTER_PX
    panel.style.maxHeight = `${Math.max(room, POPOVER_MIN_PANEL_PX)}px`
    const rect = panel.getBoundingClientRect()
    if (!rect.width) return
    let shift = 0
    const overflowRight = rect.right - (window.innerWidth - POPOVER_VIEWPORT_GUTTER_PX)
    if (overflowRight > 0) shift = -overflowRight
    const overflowLeft = POPOVER_VIEWPORT_GUTTER_PX - (rect.left + shift)
    if (overflowLeft > 0) shift += overflowLeft
    if (shift) panel.style.transform = `translateX(${shift}px)`
  }, [asSheet, open, placement])

  useEffect(() => {
    if (!open || asSheet) return
    const focusableElements = () =>
      Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
    queueMicrotask(() => {
      const firstElement = focusableElements()[0]
      if (firstElement) firstElement.focus()
      else panelRef.current?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Element | null
      // A press inside a MODAL opened from this panel is not a press outside it: the dialog is
      // portalled to the body, so closing here would unmount the control that opened it and take
      // the dialog down with it mid-confirmation.
      if (target?.closest?.('[aria-modal="true"]')) return
      // Nor is a press on a LISTBOX opened from inside this panel: `shared/ui/listbox` portals
      // its open option list to the body so no scroller can clip it, which puts the option the
      // user is choosing outside this root. Closing here would unmount the field mid-choice.
      if (target?.closest?.('[role="listbox"]')) return
      if (!rootRef.current?.contains(target)) close()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        close()
        queueMicrotask(() => triggerRef.current?.focus())
        return
      }
      if (event.key !== 'Tab') return

      const elements = focusableElements()
      const firstElement = elements[0]
      const lastElement = elements.at(-1)
      const activeElement = document.activeElement
      if (!firstElement || !lastElement) {
        event.preventDefault()
        panelRef.current?.focus()
        return
      }
      if (
        event.shiftKey &&
        (activeElement === firstElement || !panelRef.current?.contains(activeElement))
      ) {
        event.preventDefault()
        lastElement.focus()
      } else if (
        !event.shiftKey &&
        (activeElement === lastElement || !panelRef.current?.contains(activeElement))
      ) {
        event.preventDefault()
        firstElement.focus()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [asSheet, open])

  const trigger = (
    <Button
      ref={triggerRef}
      variant={triggerVariant}
      size={triggerSize}
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={open}
      // The sheet is `Sheet`'s own portalled panel and carries no id of ours, so pointing at one
      // there would be a dangling reference.
      aria-controls={open && !asSheet ? id : undefined}
      disabled={disabled}
      className={triggerClassName}
      onClick={() => setOpen((value) => !value)}
    >
      {triggerLabel ?? t('popover.options')}
    </Button>
  )

  if (asSheet) {
    return (
      <div ref={rootRef} className={twMerge('inline-flex', className)}>
        {trigger}
        <Sheet
          open={open}
          labelledBy={headingId}
          onClose={close}
          header={
            /* A visible way out, beside the name of what is open: on a phone the scrim is a
               narrow strip above a full-bleed sheet, and Escape is not a gesture a touch user
               has (§7). */
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 id={headingId} className="min-w-0 truncate text-lg font-semibold tracking-tight">
                {label}
              </h2>
              <Button variant="ghost" size="icon" aria-label={t('action.close')} onClick={close}>
                <X aria-hidden="true" className="size-5" />
              </Button>
            </div>
          }
        >
          {children(close)}
        </Sheet>
      </div>
    )
  }

  return (
    <div ref={rootRef} className={twMerge('relative inline-flex', className)}>
      {trigger}
      {open && (
        <div
          ref={panelRef}
          id={id}
          role="dialog"
          tabIndex={-1}
          aria-label={label}
          // The panel is its own scroller, so it owes the focus ring the same clear space the
          // sheet's body does (§9). Its `p-4` already exceeds the ring's 4px on every side, so
          // the gutter is satisfied by the padding it has and nothing is reserved on top of it.
          //
          // `overflow-x-hidden` is not decoration. A scroll container resolves BOTH axes away
          // from `visible`, so `overflow-y-auto` alone silently made the panel scrollable
          // SIDEWAYS as well — and a 288px surface whose content is a column of full-width fields
          // has nothing to reach out there. Anything that did stick out was a field failing to
          // shrink, which is fixed where the field is, not by letting the panel scroll. Clipping
          // happens at the padding box, so the 4px ring inside `p-4` is untouched.
          className={`bg-surface-highest max-w-popover absolute z-30 w-72 overflow-x-hidden rounded-lg p-4 shadow-lg ${
            align === 'start' ? 'left-0' : 'right-0'
          } ${
            placement === 'below'
              ? 'max-h-popover-below-max top-full mt-2 overflow-y-auto overscroll-contain'
              : 'max-h-popover-above-max bottom-full mb-2 overflow-y-auto overscroll-contain'
          }`}
        >
          {children(close)}
        </div>
      )}
    </div>
  )
})
