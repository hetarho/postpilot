import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { clsx } from 'clsx'
import { Spinner } from '../spinner/Spinner'
import { buttonStyles, type ButtonSize, type ButtonVariant } from './buttonStyles'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  /** Shows the pending state in place. The label keeps its space so the button does not resize
   *  under the thumb (design-language §6); callers should NOT also swap their own label. */
  pending?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, type = 'button', variant, size, pending = false, disabled, children, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      // Still `disabled`, so a second tap cannot fire the action while the first is in flight.
      disabled={disabled || pending}
      aria-busy={pending || undefined}
      className={buttonStyles({ variant, size, className })}
      {...props}
    >
      {/* The label stays in the layout so the box keeps the exact width it had before the press,
          and it stays in the ACCESSIBILITY TREE so the button keeps its name while busy —
          `opacity-0`, never `invisible`/`display:none`, both of which would leave a pending button
          with no accessible name at all (the spinner beside it is aria-hidden). */}
      <span className={clsx('inline-flex items-center gap-2', pending && 'opacity-0')}>
        {children}
      </span>
      {pending && (
        <span className="absolute inset-0 flex items-center justify-center">
          <Spinner />
        </span>
      )}
    </button>
  )
})
