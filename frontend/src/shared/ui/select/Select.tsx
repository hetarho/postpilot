import { forwardRef, type SelectHTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className, ...props }, ref) {
    return (
      <select
        ref={ref}
        className={twMerge(
          'bg-field-bg text-field-fg hover:bg-field-bg-hover focus:bg-field-bg-focus disabled:text-content-disabled min-h-11 w-full rounded-md px-3 py-2 text-sm disabled:opacity-50',
          className,
        )}
        {...props}
      />
    )
  },
)
