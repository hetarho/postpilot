import { forwardRef, type InputHTMLAttributes } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  appearance?: 'well' | 'bare'
}

export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(function TextField(
  { appearance = 'well', className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={twMerge(
        clsx(
          // `min-h-11` is on the BASE, not the well: the 44px touch floor is owed by both
          // appearances (design-language §4.1), and the bare editor field used to rely on its
          // caller to remember it.
          'text-field-fg placeholder:text-field-placeholder disabled:text-content-disabled min-h-11 w-full disabled:opacity-50',
          appearance === 'well'
            ? // `text-base sm:text-sm` is the `input` type role (§3.1): iOS Safari zooms the whole
              // layout when a focused control computes under 16px and never zooms back out, so the
              // phone gets 16px and the desktop keeps its density from 640px up.
              // `px-4` against the ~12px of effective vertical padding the 44px floor produces is
              // the §4.2 ratio.
              'bg-field-bg hover:bg-field-bg-hover focus:bg-field-bg-focus rounded-md px-4 py-2 text-base sm:text-sm'
            : 'bg-transparent',
          className,
        ),
      )}
      {...props}
    />
  )
})
