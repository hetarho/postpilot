import { forwardRef, type ComponentPropsWithoutRef } from 'react'
import { clsx } from 'clsx'

/** A full media hit area with a transparent resting plane and an inset focus outline. */
export const MediaAction = forwardRef<HTMLButtonElement, ComponentPropsWithoutRef<'button'>>(
  function MediaAction({ className, type = 'button', ...props }, ref) {
    return (
      <button
        ref={ref}
        type={type}
        className={clsx(
          'active:bg-media-scrim-bg/20 min-h-11 min-w-11 bg-transparent focus-visible:-outline-offset-2 active:brightness-90 disabled:active:bg-transparent disabled:active:brightness-100 aria-disabled:active:bg-transparent aria-disabled:active:brightness-100',
          className,
        )}
        {...props}
      />
    )
  },
)
