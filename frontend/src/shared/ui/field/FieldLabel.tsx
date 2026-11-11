import { forwardRef, type LabelHTMLAttributes } from 'react'
import { twMerge } from '@/shared/lib'

export const FieldLabel = forwardRef<HTMLLabelElement, LabelHTMLAttributes<HTMLLabelElement>>(
  function FieldLabel({ className, ...props }, ref) {
    return (
      <label
        ref={ref}
        className={twMerge('text-field-label block text-sm font-medium', className)}
        {...props}
      />
    )
  },
)
