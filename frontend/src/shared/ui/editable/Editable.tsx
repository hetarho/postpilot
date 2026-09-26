import { useState, type ReactNode } from 'react'
import { Pencil } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { Button } from '../button/Button'

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
  /** The pencil's accessible name. An icon-only button has no other name (§9), and it has to name
   *  the field, since a screen full of pencils named "수정" identifies nothing. */
  editLabel: string
  edit: (exit: () => void) => ReactNode
  children: ReactNode
  className?: string
  /** Keeps the read presentation but removes the mutation affordance. */
  readOnly?: boolean
}) {
  const [editing, setEditing] = useState(false)
  if (editing && !readOnly) return <div className={className}>{edit(() => setEditing(false))}</div>
  return (
    <div className={twMerge('flex items-start gap-2', className)}>
      <div className="min-w-0 flex-1">{children}</div>
      {!readOnly && (
        <Button
          variant="ghost"
          size="icon"
          aria-label={editLabel}
          onClick={() => setEditing(true)}
          className="shrink-0"
        >
          <Pencil className="size-4" aria-hidden />
        </Button>
      )}
    </div>
  )
}
