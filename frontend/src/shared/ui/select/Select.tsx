import { forwardRef, type SelectHTMLAttributes } from 'react'
import { ChevronDown } from 'lucide-react'
import { twMerge } from 'tailwind-merge'

/** A bounded choice in a form: a native `<select>` wearing the field well, so the platform picker
 *  and its accessibility come for free (design-language §7).
 *
 *  The chevron is drawn because the well alone is not a sufficient signal on touch: `hover:` never
 *  matches on a phone, so without it a Select and a TextField are the same rectangle and the user
 *  has to guess whether tapping opens a picker or a keyboard. */
export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  function Select({ className, ...props }, ref) {
    return (
      <span className="relative block">
        <select
          ref={ref}
          className={twMerge(
            'bg-field-bg text-field-fg hover:bg-field-bg-hover focus:bg-field-bg-focus disabled:text-content-disabled min-h-11 w-full appearance-none rounded-md py-2 pr-11 pl-4 text-base disabled:opacity-50 sm:text-sm',
            className,
          )}
          {...props}
        />
        <ChevronDown
          aria-hidden="true"
          className="text-content-tertiary pointer-events-none absolute top-1/2 right-4 size-4 -translate-y-1/2"
        />
      </span>
    )
  },
)
