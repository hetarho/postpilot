import { forwardRef, type ButtonHTMLAttributes } from 'react'
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
      {/* The label stays in the layout — `invisible`, not unmounted — so the box keeps the exact
          width it had before the press. The spinner is centred over it. */}
      <span className={pending ? 'invisible contents' : 'contents'}>{children}</span>
      {pending && (
        <span className="absolute inset-0 flex items-center justify-center">
          <Spinner />
        </span>
      )}
    </button>
  )
})
