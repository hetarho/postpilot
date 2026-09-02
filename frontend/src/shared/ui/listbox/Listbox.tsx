import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { Check, ChevronDown } from 'lucide-react'
import {
  LISTBOX_MAX_VIEWPORT_RATIO,
  LISTBOX_MIN_PANEL_PX,
  LISTBOX_TRIGGER_GAP_PX,
  LISTBOX_VIEWPORT_GUTTER_PX,
} from '@/shared/config'

export interface ListboxOption<T> {
  value: T
  label: string
  /** Listed but not choosable. `aria-disabled`, not `disabled`, so the arrow keys still reach it
   *  and a screen reader still reads the entry whose whole purpose is to say why it is unusable. */
  disabled?: boolean
}

interface ListboxProps<T> {
  /** Goes on the TRIGGER, so a caller's `FieldLabel htmlFor` still points at the control. */
  id?: string
  value: T
  options: readonly ListboxOption<T>[]
  onChange: (value: T) => void
  /** Shown when `value` matches no option — the empty state of a field with no choice yet. */
  placeholder?: string
  disabled?: boolean
  className?: string
  /** id of the visible `FieldLabel`. The trigger's name becomes "<label> <current option>", which
   *  is what a native select announced. */
  'aria-labelledby'?: string
  'aria-label'?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
}

/** Where the open panel is painted, in viewport coordinates. It is `position: fixed` at the
 *  document body, so these are the measured trigger's own numbers rather than an offset inside
 *  some containing block. */
interface PanelBox {
  left: number
  width: number
  /** Set for a panel that drops DOWN — the distance from the viewport's top edge. */
  top?: number
  /** Set for a panel that flipped UP — the distance from the viewport's bottom edge. */
  bottom?: number
  maxHeight: number
  drop: 'down' | 'up'
}

function samePanelBox(a: PanelBox, b: PanelBox): boolean {
  return (
    a.left === b.left &&
    a.width === b.width &&
    a.top === b.top &&
    a.bottom === b.bottom &&
    a.maxHeight === b.maxHeight &&
    a.drop === b.drop
  )
}

/** The app-drawn replacement for the native select element (design-language §7, owner decision
 *  2026-08-31). The OS draws a native select's open option list, so it can wear neither the app's
 *  surfaces nor its type roles and it visibly leaves the design system the moment it opens.
 *
 *  It is NOT `Menu`: that primitive is `aria-haspopup="menu"` behind an icon-only trigger, which
 *  is the wrong role and the wrong shape for a labelled form field.
 *
 *  WAI-APG listbox-button semantics: the trigger is the only tab stop, options are programmatic
 *  focus targets (`tabIndex={-1}`) inside a `role="listbox"` panel, and the check marks the
 *  current value. Options are ordinary buttons, so Enter and Space select through the native
 *  click; Escape and an outside press close and return focus to the trigger.
 *
 *  The open panel is PORTALLED to the document body and positioned from the trigger's measured
 *  rect. As a sibling of its trigger it was clipped by every scrolling ancestor — worst inside
 *  `Sheet`, whose body is the sheet's one scroller, and worst again for a field near the bottom,
 *  which is exactly the field whose panel flips upward and out of that scroller. A portal has no
 *  ancestor to be clipped by, at the cost of having to track the trigger itself: the panel
 *  re-measures on scroll and resize, and CLOSES rather than detaching once the trigger leaves the
 *  viewport.
 *
 *  Generic over the option value rather than over a string, so a numeric enum (a block type, a
 *  heading level) round-trips without a stringly-typed cast at every call site. */
export function Listbox<T>({
  id,
  value,
  options,
  onChange,
  placeholder,
  disabled = false,
  className,
  'aria-labelledby': labelledBy,
  'aria-label': ariaLabel,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
}: ListboxProps<T>) {
  const generatedId = useId()
  const panelId = `${generatedId}-panel`
  const valueId = `${generatedId}-value`
  const [open, setOpen] = useState(false)
  const [box, setBox] = useState<PanelBox>()
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  const selected = options.find((option) => option.value === value)

  const items = () =>
    Array.from(panelRef.current?.querySelectorAll<HTMLButtonElement>('[role="option"]') ?? [])

  const close = (returnFocus: boolean, preventScroll = false) => {
    setOpen(false)
    if (returnFocus) queueMicrotask(() => triggerRef.current?.focus({ preventScroll }))
  }

  const select = (option: ListboxOption<T>) => {
    if (option.disabled) return
    close(true)
    if (option.value !== value) onChange(option.value)
  }

  // A trigger can sit anywhere in the viewport, and nothing in CSS knows where — a `dvh` ceiling
  // is the same number for a field at the top of the page and for one in the docked bar, where a
  // panel half the screen tall opens entirely past the bottom edge. So the room is MEASURED: the
  // panel opens downward into the gap it actually has, flips above the trigger when that gap is
  // too small and the one overhead is larger, and scrolls inside whichever it took. The ratio
  // caps it where there is more room than a list needs; the floor keeps it usable rather than
  // squeezing it to a sliver for a trigger pinned against an edge.
  //
  // Since the panel is portalled, the same measurement also supplies its POSITION, and it has to
  // be repeated for as long as the panel is open: a page or overlay scroll moves the trigger out
  // from under a panel that no longer travels with it.
  useLayoutEffect(() => {
    const trigger = triggerRef.current
    if (!open || !trigger) return
    const measure = () => {
      const anchor = trigger.getBoundingClientRect()
      // Gone from the viewport, which happens when its own scroller carries it away. A panel
      // anchored to nothing is worse than no panel, so it closes — but without stealing focus
      // back, since a scroll is not a dismissal the user aimed at the trigger.
      //
      // `height > 0` is what separates "laid out, and scrolled out of sight" from "not laid out
      // at all": scrolling never changes a rect's size, while an environment with no layout engine
      // reports every rect as zero. Without it the panel would refuse to open in jsdom — and in
      // any browser during the frame before layout.
      const gone =
        anchor.height > 0 &&
        (anchor.bottom <= 0 ||
          anchor.top >= window.innerHeight ||
          anchor.right <= 0 ||
          anchor.left >= window.innerWidth)
      if (gone) {
        // Focus is on an OPTION at this point (the panel takes it on open), and that node is about
        // to be unmounted — leaving `document.body` focused, which is outside any sheet's trap and
        // lets the next Tab escape the modal. So focus goes back to the trigger even though the
        // user did not dismiss anything, with `preventScroll` so returning it does not undo the
        // scroll that caused this.
        close(panelRef.current?.contains(document.activeElement) ?? false, true)
        return
      }
      const gap = LISTBOX_TRIGGER_GAP_PX + LISTBOX_VIEWPORT_GUTTER_PX
      const below = window.innerHeight - anchor.bottom - gap
      const above = anchor.top - gap
      const flip = below < LISTBOX_MIN_PANEL_PX && above > below
      const room = Math.min(flip ? above : below, window.innerHeight * LISTBOX_MAX_VIEWPORT_RATIO)
      const next: PanelBox = {
        drop: flip ? 'up' : 'down',
        left: anchor.left,
        width: anchor.width,
        top: flip ? undefined : anchor.bottom + LISTBOX_TRIGGER_GAP_PX,
        bottom: flip ? window.innerHeight - anchor.top + LISTBOX_TRIGGER_GAP_PX : undefined,
        maxHeight: Math.max(room, LISTBOX_MIN_PANEL_PX),
      }
      // Kept BY VALUE. Callers pass their options inline, so this effect re-runs on every render
      // of the field's parent; a fresh object each time would be a new state on every render and
      // a new render on every state. It is also what keeps a scroll from re-rendering the panel
      // once the trigger has stopped moving.
      setBox((current) => (current && samePanelBox(current, next) ? current : next))
    }
    measure()
    // Capture, so a scroll inside ANY ancestor — a sheet's body, the page — is seen; `scroll`
    // does not bubble.
    window.addEventListener('scroll', measure, true)
    window.addEventListener('resize', measure)
    return () => {
      window.removeEventListener('scroll', measure, true)
      window.removeEventListener('resize', measure)
    }
  }, [open, options])

  useEffect(() => {
    if (!open) return
    // The current option receives focus so the keyboard arrives where the state already is —
    // read off the rendered rows so the effect depends on nothing but being open.
    queueMicrotask(() => {
      const elements = items()
      const current = elements.find((element) => element.getAttribute('aria-selected') === 'true')
      ;(current ?? elements[0])?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null
      // The panel is no longer a descendant of the root, so it has to be named as inside
      // explicitly — otherwise choosing an option IS the outside press that closes the panel.
      if (rootRef.current?.contains(target) || panelRef.current?.contains(target)) return
      close(true)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  const onPanelKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      // The listbox may be open INSIDE an overlay that also closes on Escape (the writing brief's
      // popover, a sheet). One Escape must dismiss one thing — the innermost one. React delivers
      // portalled events through the tree, and this stops the native event at the portal
      // container, before it reaches the document listeners those overlays install.
      event.stopPropagation()
      close(true)
      return
    }
    if (event.key === 'Tab') {
      // Do not prevent the native traversal, but move focus to the TRIGGER first, synchronously.
      // The panel now lives at the end of the document, so leaving focus on an option would send
      // Tab off the end of the page instead of to the control after the field. From the trigger,
      // forward Tab reaches the next control and Shift+Tab the previous one — and any focus trap
      // the field sits inside (a sheet's) sees a target it still owns.
      triggerRef.current?.focus()
      setOpen(false)
      return
    }
    if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      items()
        .at(event.key === 'Home' ? 0 : -1)
        ?.focus()
      return
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const elements = items()
    if (!elements.length) return
    const step = event.key === 'ArrowDown' ? 1 : -1
    const current = elements.indexOf(document.activeElement as HTMLButtonElement)
    elements[(current + step + elements.length) % elements.length]?.focus()
  }

  const onTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    setOpen(true)
  }

  return (
    <div ref={rootRef} className={twMerge('relative', className)}>
      <button
        ref={triggerRef}
        type="button"
        id={id}
        disabled={disabled}
        // `combobox`, which is the role a native select exposed, over the plain button a
        // menu-style trigger would be: this is a form field whose value is one of a list, and
        // assistive technology should keep announcing it as one.
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        aria-labelledby={labelledBy ? `${labelledBy} ${valueId}` : undefined}
        aria-label={ariaLabel}
        aria-describedby={describedBy}
        aria-invalid={invalid}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
        // The field well the native select wore, minus the native control. `px-4` against the
        // ~12px the 44px floor produces is the §4.2 ratio; the type is the `input` role (§3.1).
        className="bg-field-bg text-field-fg hover:bg-field-bg-hover focus:bg-field-bg-focus disabled:text-content-disabled flex min-h-11 w-full items-center gap-2 rounded-md px-4 py-2 text-left text-base disabled:opacity-50 sm:text-sm"
      >
        <span
          id={valueId}
          className={clsx('min-w-0 flex-1 truncate', !selected && 'text-field-placeholder')}
        >
          {selected?.label ?? placeholder ?? ''}
        </span>
        {/* The well alone is not a signal on touch: `hover:` never matches on a phone, so without
            the chevron a Listbox and a TextField are the same rectangle and the user has to guess
            whether tapping opens a picker or a keyboard. */}
        <ChevronDown aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
      </button>
      {open &&
        box &&
        createPortal(
          <div
            ref={panelRef}
            id={panelId}
            role="listbox"
            aria-label={ariaLabel}
            aria-labelledby={labelledBy}
            data-drop={box.drop}
            onKeyDown={onPanelKeyDown}
            // `bg-surface-overlay`, NOT `surface-highest`: that is the token `Sheet`'s own panel
            // wears, so an open list inside a sheet was the same colour as the plane behind it
            // with only a shadow between them (§2.1). The sixth surface step exists for this.
            //
            // An overlay with its own bounds is §4.4's dropdown/sheet exception, so this one
            // nested scroller is allowed: a forty-model catalog otherwise pushes the page past
            // the field it belongs to. Its ceiling and its position are the measured ones set
            // above, not tokens — see that effect.
            className="bg-surface-overlay z-overlay-panel fixed overflow-y-auto overscroll-contain rounded-lg p-1 shadow-lg select-none"
            style={{
              left: box.left,
              width: box.width,
              top: box.top,
              bottom: box.bottom,
              maxHeight: box.maxHeight,
            }}
          >
            {options.map((option) => {
              const current = option.value === value
              return (
                <button
                  key={String(option.value)}
                  type="button"
                  role="option"
                  aria-selected={current}
                  aria-disabled={option.disabled || undefined}
                  tabIndex={-1}
                  onClick={() => select(option)}
                  className={clsx(
                    'flex min-h-11 w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors',
                    option.disabled
                      ? 'text-content-disabled'
                      : clsx(
                          'hover:bg-row-bg-hover active:bg-row-bg-active',
                          current ? 'text-content-primary font-medium' : 'text-content-secondary',
                        ),
                  )}
                >
                  {/* Wraps rather than truncating: an option's label carries the badges and the
                      reason a model cannot be chosen, and the open panel is the one place there is
                      room to read them (§7). */}
                  <span className="min-w-0 flex-1 break-words">{option.label}</span>
                  {/* The unchecked check keeps its box so labels align and the panel width is
                      stable as the value moves. */}
                  <Check
                    aria-hidden="true"
                    className={clsx('size-4 shrink-0', !current && 'opacity-0')}
                  />
                </button>
              )
            })}
          </div>,
          document.body,
        )}
    </div>
  )
}
