import { forwardRef, type InputHTMLAttributes } from 'react'
import { Check } from 'lucide-react'
import { twMerge } from 'tailwind-merge'

/** A 20px box with a real 44px hit area. The PRIMITIVE owes the touch target, not the caller
 *  (design-language §4.1): a bare 20x20 input is 21% of the area a thumb needs, and relying on
 *  every future call site to remember a 44px wrapper is how the rule gets lost.
 *
 *  The native input is kept — it is the control, with all of its keyboard and assistive-technology
 *  behaviour — but made transparent and stretched to 44px over a drawn box. A pseudo-element would
 *  not have worked: `<input>` is a replaced element, so `::before` never renders on it. */
export const Checkbox = forwardRef<
  HTMLInputElement,
  Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>
>(function Checkbox({ className, ...props }, ref) {
  return (
    <span className={twMerge('relative inline-flex size-5 shrink-0', className)}>
      <input
        ref={ref}
        type="checkbox"
        // `-inset-3` grows the invisible control 12px on every side — 20 + 24 = 44 — without
        // changing the 20px of layout the visual box occupies, so no caller's row re-flows.
        className="peer absolute -inset-3 z-10 opacity-0"
        {...props}
      />
      <span
        aria-hidden="true"
        // The glyph is always rendered and only its colour changes, because `peer-checked:` reaches
        // this sibling but not its descendants — so the tick rides on `currentColor`.
        className="bg-field-bg peer-checked:bg-button-cta-bg peer-checked:text-button-cta-fg peer-focus-visible:outline-focus-ring inline-flex size-5 items-center justify-center rounded-sm text-transparent transition-colors peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-disabled:opacity-50"
      >
        <Check className="size-3.5" strokeWidth={3} />
      </span>
    </span>
  )
})
