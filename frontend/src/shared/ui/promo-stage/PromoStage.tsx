import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** The ground a promotional surface stands on: a recessed panel with an aurora drifting behind
 *  whatever it holds (THEME-37).
 *
 *  Three blurred blobs of the promotional hues, each on its own slow period and heading, clipped
 *  to the panel so the weather stays behind the options and never leaks onto the page around
 *  them. They move on `transform` alone — blur is applied once, not animated — so the whole
 *  backdrop is compositor work, and the global reduced-motion rule holds the three at their first
 *  frame: a still aurora rather than a blank panel.
 *
 *  It is one of the three promotional primitives and, like `PromoFrame`, it is named for the
 *  exemption it carries: a reviewer who sees `PromoStage` under an ordinary list has found the
 *  exception spreading, not a new pattern. */
export function PromoStage({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={twMerge('relative isolate rounded-xl p-4 sm:p-6', className)}>
      <div
        aria-hidden="true"
        data-promo-aurora=""
        // The panel's own plane is one step below the page, so the cards on `surface-raised`
        // read as standing on a stage, and the clip is on THIS layer rather than the root so a
        // marked card's halo and scale step are not cut at the panel's edge.
        className="bg-surface-recessed pointer-events-none absolute inset-0 -z-10 overflow-hidden rounded-xl"
      >
        <div className="bg-promo-aurora-1 animate-aurora-1 absolute -top-1/4 -left-1/6 size-2/3 rounded-full opacity-45 blur-3xl" />
        <div className="bg-promo-aurora-2 animate-aurora-2 absolute top-1/4 -right-1/4 size-2/3 rounded-full opacity-40 blur-3xl" />
        <div className="bg-promo-aurora-3 animate-aurora-3 absolute -bottom-1/3 left-1/4 size-1/2 rounded-full opacity-35 blur-3xl" />
      </div>
      {children}
    </div>
  )
}
