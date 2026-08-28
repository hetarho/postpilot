import { forwardRef, type HTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

export const FieldMessage = forwardRef<HTMLParagraphElement, HTMLAttributes<HTMLParagraphElement>>(
  function FieldMessage({ className, role = 'alert', ...props }, ref) {
    return (
      <p
        ref={ref}
        role={role}
        className={twMerge('text-field-error text-sm', className)}
        {...props}
      />
    )
  },
)
