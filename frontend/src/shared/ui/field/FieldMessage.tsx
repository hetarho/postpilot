import { forwardRef, type HTMLAttributes } from 'react'
import { twMerge } from '@/shared/lib'

export const FieldMessage = forwardRef<HTMLParagraphElement, HTMLAttributes<HTMLParagraphElement>>(
  function FieldMessage({ className, role = 'alert', ...props }, ref) {
    return (
      <p
        ref={ref}
        role={role}
        className={twMerge('text-field-error text-sm break-words', className)}
        {...props}
      />
    )
  },
)
