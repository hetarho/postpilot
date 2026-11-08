import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Pencil } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { Button } from '../button/Button'
import { FOCUSABLE_SELECTOR, focusablesIn } from '../focus/focusable'

/** Read first, edit on request: a value renders as text until its edit control is pressed.
 *
 *  This primitive owns ONLY the toggle and the pencil affordance — the caller supplies the read
 *  view, the edit view, and every action inside it. That split is what lets one shape serve a
 *  profile field and an editor block without either domain leaking into `shared/ui`.
 *
 *  `edit` receives the function that leaves edit mode, so the caller decides what counts as done:
 *  a successful save exits, a rejected one stays open with the draft intact. */
export function Editable({
  editLabel,
  edit,
  children,
  className,
  readOnly = false,
}: {
  /** The pencil's accessible name. An icon-only button has no other name (THEME-33), and it has to name
   *  the field, since a screen full of pencils named "수정" identifies nothing. */
  editLabel: string
  edit: (exit: () => void) => ReactNode
  children: ReactNode
  className?: string
  /** Keeps the read presentation but removes the mutation affordance. */
  readOnly?: boolean
}) {
  // Each toggle swaps the focused control out of the document, which drops focus to <body> and
  // sends a keyboard user back to the top of the page, so a toggle carries the focus move it owes.
  // State, not a ref: `exit` is handed to `edit` during render.
  const [toggle, setToggle] = useState<{ editing: boolean; move: 'enter' | 'exit' | null }>({
    editing: false,
    move: null,
  })
  const { editing } = toggle
  const trigger = useRef<HTMLButtonElement>(null)
  const view = useRef<HTMLDivElement>(null)

  useEffect(() => {
    // Only while focus is lost: React has already focused an autoFocus field by now, and focus the
    // user moved elsewhere before a late exit stays where they put it.
    const active = document.activeElement
    if (!toggle.move || (active && active !== document.body)) return
    if (toggle.move === 'exit') {
      // A collapsing edit card must not scroll the page to the pencil.
      trigger.current?.focus({ preventScroll: true })
      return
    }
    // Sheet's convention: `data-autofocus` marks the control to start on, or a wrapper of it.
    const marked = view.current?.querySelector<HTMLElement>('[data-autofocus]')
    const start = marked?.matches(FOCUSABLE_SELECTOR) ? marked : focusablesIn(marked)[0]
    ;(start ?? focusablesIn(view.current)[0])?.focus()
  }, [toggle])

  // Leaving when already left changes nothing, as before.
  const exit = () =>
    setToggle((current) => (current.editing ? { editing: false, move: 'exit' } : current))

  if (editing && !readOnly)
    return (
      <div ref={view} className={className}>
        {edit(exit)}
      </div>
    )
  return (
    <div className={twMerge('flex items-start gap-2', className)}>
      <div className="min-w-0 flex-1">{children}</div>
      {!readOnly && (
        <Button
          ref={trigger}
          variant="ghost"
          size="icon"
          aria-label={editLabel}
          // `detail` 0 is a keyboard activation. A tap moves no focus: a field focused by script
          // after one would raise the phone keyboard over the card it just opened.
          onClick={(event) =>
            setToggle({ editing: true, move: event.detail === 0 ? 'enter' : null })
          }
          className="shrink-0"
        >
          <Pencil className="size-4" aria-hidden />
        </Button>
      )}
    </div>
  )
}
