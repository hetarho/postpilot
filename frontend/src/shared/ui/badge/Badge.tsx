import { forwardRef, type HTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

export const Badge = forwardRef<HTMLSpanElement, HTMLAttributes<HTMLSpanElement>>(function Badge(
  { className, ...props },
  ref,
) {
  return (
    <span
      ref={ref}
      className={twMerge(
        'bg-badge-neutral-bg text-badge-neutral-fg inline-flex items-center rounded-sm px-1.5 py-0.5 text-xs font-medium',
        className,
      )}
      {...props}
    />
  )
})
