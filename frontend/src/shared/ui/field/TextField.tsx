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
          'text-field-fg placeholder:text-field-placeholder disabled:text-content-disabled w-full disabled:opacity-50',
          appearance === 'well'
            ? 'bg-field-bg hover:bg-field-bg-hover focus:bg-field-bg-focus min-h-11 rounded-md px-3 py-2 text-sm'
            : 'bg-transparent',
          className,
        ),
      )}
      {...props}
    />
  )
})
