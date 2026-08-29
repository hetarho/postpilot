import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** Docks a view's committing actions in the thumb's band instead of leaving them wherever the
 *  document flow put them (design-language §4.3). Lifted out of
 *  `features/review-model-experiment`, which hand-rolled this shape — the second slice needing it
 *  is what §1.1 says makes it a primitive.
 *
 *  It sticks ABOVE the phone tab bar (`bottom-nav`) and against the bottom edge from `sm:` up
 *  where that bar does not exist. The padding is what keeps it off the very edge: NN/g's
 *  bottom-sheet research is explicit that the extreme bottom is not the most reachable region, so
 *  a docked bar with real padding is the shape that satisfies both that and the platform tab-bar
 *  convention. */
export function ActionBar({
  children,
  className,
  ariaLabel,
}: {
  children: ReactNode
  className?: string
  ariaLabel?: string
}) {
  return (
    <div
      aria-label={ariaLabel}
      className={twMerge(
        'bg-surface-highest bottom-nav sm:pb-safe-b sticky z-20 mt-6 rounded-xl p-4 shadow-lg sm:bottom-0',
        className,
      )}
    >
      {children}
    </div>
  )
}
