import { forwardRef, type InputHTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

/** A two-state switch: ON means this row's value comes from OUTSIDE the text (THEME-29). It is
 *  deliberately not a `Checkbox` — a checkbox says "selected", and 데이터 받기 says "the post's
 *  author fills this in" (TEMPLATE-44).
 *
 *  The native input is kept and made transparent over a drawn track, exactly as `Checkbox` does
 *  it: it is the control, with all of its keyboard and assistive-technology behaviour, plus
 *  `role="switch"` so it is announced as on/off rather than checked. `<input>` is a replaced
 *  element, so a pseudo-element would never render on it.
 *
 *  The track and the knob are both SIBLINGS of the input, because `peer-checked:` reaches a
 *  sibling and not its descendants — a knob nested inside the track would never move.
 *
 *  The visible box is 24 × 40, above WCAG 2.5.8's 24 px floor under a fine pointer; under a
 *  coarse one the primitive grows its own hit area past 44 px rather than making every call site
 *  remember to (THEME-23). */
export const Switch = forwardRef<
  HTMLInputElement,
  Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'role'>
>(function Switch({ className, ...props }, ref) {
  return (
    <span className={twMerge('relative inline-flex h-6 w-10 shrink-0', className)}>
      <input
        ref={ref}
        type="checkbox"
        role="switch"
        // `-inset-2.5` grows the invisible control 10px on every side — 24 + 20 = 44 — without
        // changing the 24px of layout the visible track occupies, so no caller's row re-flows.
        className="peer absolute inset-0 z-10 opacity-0 pointer-coarse:-inset-2.5"
        {...props}
      />
      <span
        aria-hidden="true"
        className="bg-field-bg peer-checked:bg-button-cta-bg peer-focus-visible:outline-focus-ring h-6 w-10 rounded-full transition-colors peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-disabled:opacity-50"
      />
      <span
        aria-hidden="true"
        className="bg-surface-highest absolute top-0.5 left-0.5 size-5 rounded-full transition-transform peer-checked:translate-x-4 peer-disabled:opacity-50"
      />
    </span>
  )
})
