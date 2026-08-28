import { forwardRef, type TextareaHTMLAttributes } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  appearance?: 'well' | 'bare'
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { appearance = 'well', className, ...props },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={twMerge(
        clsx(
          'text-field-fg placeholder:text-field-placeholder disabled:text-content-disabled w-full resize-none disabled:opacity-50',
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
