import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

/** A chip that is pressed or not: one option in a set where any number may be chosen at once, or
 *  — where the caller releases the others as it presses one, as ①'s 분야 does — exactly one.
 *
 *  It is a `button` with `aria-pressed` rather than a checkbox, because a set of them is a
 *  row of tappable labels rather than a form field, and assistive technology should announce
 *  each one as pressed or not pressed rather than as part of a group with a legend.
 *
 *  Pressed takes the accent BADGE pair rather than the CTA's fill: the accent area belongs to
 *  the one action the screen wants taken next, and a dozen chips filled with it would drown
 *  the confirm button they sit above (THEME-18). Nothing about the box changes size between
 *  the two states, so a row of chips never reflows as they are pressed.
 *
 *  Under a coarse pointer the chip grows its own hit area past the 44 px floor rather than
 *  making every call site remember to (THEME-23). */
export const ChipToggle = forwardRef<
  HTMLButtonElement,
  Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'type' | 'aria-pressed'> & { pressed: boolean }
>(function ChipToggle({ className, pressed, ...props }, ref) {
  return (
    <button
      ref={ref}
      type="button"
      aria-pressed={pressed}
      className={twMerge(
        // No `focus-visible:ring-*`: the global `:focus-visible` outline already draws one,
        // and a second ring on top of it reads as two states at once.
        'inline-flex min-h-9 shrink-0 items-center rounded-sm px-3 text-sm font-medium',
        'duration-fast ease-standard transition-colors pointer-coarse:min-h-11',
        pressed
          ? 'bg-badge-accent-bg text-badge-accent-fg'
          : 'bg-surface-recessed text-content-secondary hover:bg-row-bg-hover hover:text-content-primary active:bg-row-bg-active',
        className,
      )}
      {...props}
    />
  )
})
