import { useId, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import { Checkbox } from '../checkbox/Checkbox'
import { listRowStyles } from '../list-group/listRowStyles'
import { Spinner } from '../spinner/Spinner'
import { SurfaceButton } from '../surface-button/SurfaceButton'
import { typographyStyles } from '../typography/typographyStyles'

/** One compact row of a choice list: its name, an optional mark beside the name, and one meta line
 *  — what the choice costs, how far it has come, or why it cannot be chosen.
 *
 *  `radio` is one pick among peers: the whole row is the button and the round mark at its end
 *  fills when it is the one. `checkbox` is one of several that are on or off: the row is the
 *  checkbox's label. A `disabled` row stays listed so its meta can say why; a checked checkbox row
 *  stays uncheckable even then, so a choice that is on can always be taken back. `pending` is the
 *  row's own save in flight: the mark gives way to a spinner and a press does nothing, while the
 *  row keeps its focus — a disabled control would drop it to the page. */
export function ChoiceRow({
  mode,
  checked,
  name,
  mark,
  meta,
  disabled = false,
  pending = false,
  onPick,
  className,
}: {
  mode: 'radio' | 'checkbox'
  checked: boolean
  name: ReactNode
  mark?: ReactNode
  meta?: ReactNode
  disabled?: boolean
  pending?: boolean
  onPick: () => void
  className?: string
}) {
  const id = useId()
  const nameId = `${id}-name`
  const metaId = `${id}-meta`
  const text = (
    <span className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
      <span className="flex min-w-0 items-center gap-2">
        <span id={nameId} className="min-w-0 break-words">
          {name}
        </span>
        {mark}
      </span>
      {meta && (
        <span id={metaId} className={typographyStyles({ variant: 'meta' })}>
          {meta}
        </span>
      )}
    </span>
  )
  if (mode === 'checkbox')
    return (
      <label
        aria-busy={pending || undefined}
        className={listRowStyles({
          interactive: !disabled && !pending,
          className: clsx(
            'min-h-12 py-2',
            disabled || pending ? 'cursor-not-allowed' : 'cursor-pointer',
            disabled && 'opacity-60',
            className,
          ),
        })}
      >
        <Checkbox
          checked={checked}
          disabled={disabled && !checked}
          aria-labelledby={nameId}
          aria-describedby={meta ? metaId : undefined}
          onChange={() => !pending && onPick()}
        />
        {text}
        {pending && <Spinner />}
      </label>
    )
  return (
    <SurfaceButton
      presentation="row"
      role="radio"
      aria-checked={checked}
      aria-busy={pending || undefined}
      aria-labelledby={meta ? `${nameId} ${metaId}` : nameId}
      disabled={disabled}
      onClick={() => !pending && onPick()}
      className={listRowStyles({
        interactive: !disabled,
        className: clsx('min-h-12 rounded-none py-2 disabled:cursor-not-allowed', className),
      })}
    >
      {text}
      {pending ? (
        // In the mark's own 24px, so the row does not shift while its save runs.
        <span className="flex size-6 shrink-0 items-center justify-center">
          <Spinner />
        </span>
      ) : (
        <span
          aria-hidden="true"
          className={clsx(
            'flex size-6 shrink-0 items-center justify-center rounded-full',
            checked ? 'bg-button-cta-bg text-button-cta-fg' : 'bg-progress-step-todo',
          )}
        >
          {checked && <Check className="size-4" />}
        </span>
      )}
    </SurfaceButton>
  )
}
