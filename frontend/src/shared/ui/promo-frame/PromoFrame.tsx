import type { ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'

/** A promotional frame: content on its own surface, inside a stroke of accent gradient that
 *  turns slowly behind it.
 *
 *  It is the one place this design language allows a gradient and the one border outside the
 *  structural exceptions, and THEME-37 scopes both to a surface whose job is to be chosen
 *  from — the plan ladder and its estimator, nothing else. The name says so on purpose: a
 *  reviewer should see the exception being reused rather than a decoration spreading.
 *
 *  The stroke is a square layer twice the frame's width, rotating behind an opaque inner
 *  element that covers all but the padding. Only `transform` moves, so four of these are
 *  compositor work rather than layout, and the global reduced-motion rule freezes them at
 *  their first frame — a still gradient stroke, not a missing one.
 *
 *  `marked` is the one option a view recommends: a wider stroke and a shadow. What it MEANS is
 *  always carried by a text label beside it, never by the glow. */
export function PromoFrame({
  children,
  marked,
  className,
}: {
  children: ReactNode
  marked?: boolean
  className?: string
}) {
  return (
    <div
      className={twMerge(
        'relative isolate h-full overflow-hidden rounded-lg',
        marked ? 'p-0.5 shadow-md' : 'p-px',
        className,
      )}
    >
      <div
        aria-hidden="true"
        data-promo-stroke=""
        // The centring translate is on the class AND in the keyframes: an animation's transform
        // replaces the class's, and a browser running no animation at all would otherwise leave
        // the layer covering one quadrant.
        className="bg-promo-stroke animate-promo-spin w-promo-layer absolute top-1/2 left-1/2 aspect-square -translate-x-1/2 -translate-y-1/2"
      />
      {/* Opaque and above the layer, so the gradient shows only in the padding. The content
          keeps the surface pair every figure inside it was audited against (THEME-7). */}
      <div className="bg-surface-raised relative h-full rounded-lg p-4">{children}</div>
    </div>
  )
}
