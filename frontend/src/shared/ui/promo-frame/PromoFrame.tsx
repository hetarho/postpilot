import { useRef, type CSSProperties, type PointerEvent, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { PROMO_TILT_MAX_DEG, PROMO_TILT_PERSPECTIVE_PX } from '@/shared/config'
import { prefersReducedMotion } from '@/shared/lib'

/** A promotional frame: content on its own surface, inside a still stroke of accent gradient,
 *  that TILTS toward a fine pointer and carries a spotlight under it.
 *
 *  It is one of the three promotional primitives (THEME-37): the exemption from the design
 *  language's restraint is scoped to the plan ladder, the estimator, `/about`'s plans and any
 *  landing section whose job is to get one option chosen — and the name says so on purpose, so a
 *  reviewer sees the exception being reused rather than a decoration spreading.
 *
 *  The stroke is the gradient painted on a box whose padding is the stroke's width, under an
 *  opaque inner element that covers all but that padding. It does not move: the earlier version
 *  turned a highlight around the edge, and a light travelling along the top of a border read as a
 *  loading indicator rather than a finish (owner decision 2026-09-09).
 *
 *  The TILT is the card answering the hand: as a mouse crosses it the card rotates a few degrees
 *  toward the pointer, seen from a long perspective so it reads as a card catching the light and
 *  not as a panel falling over, and settles flat when the pointer leaves. Only `transform` moves,
 *  and the frame's own `transition-transform` smooths every step, so the tilt lags the pointer by
 *  a beat the way a physical card would. A touch never tilts — a finger covers what it presses,
 *  and there is no hover to leave — and reduced motion leaves the card flat. The SPOTLIGHT is a
 *  soft accent disc centred on the pointer, faded in under `hover:` — which Tailwind compiles to
 *  `(hover: hover)`, so a touchscreen never pays for it. Both the angle and the spot are written
 *  straight into styles on the frame rather than into React state: a pointer moves at 60–120 Hz
 *  and a render per event would make the card the slowest thing on the page for no visible gain.
 *
 *  `marked` is the one option a view recommends: a wider stroke, a shadow, and a HALO — the same
 *  gradient once more, blurred and breathing behind the card, which is what lets the marked
 *  option glow past its own edge instead of only being outlined a little thicker. The halo
 *  animates opacity alone, so it is compositor work and the global reduced-motion rule leaves it
 *  still rather than absent. What `marked` MEANS is always carried by a text label beside it,
 *  never by the glow.
 *
 *  The frame lifts a step under a fine pointer: a card that is asking to be chosen answers the
 *  hover the way a button does, and `translate` is compositor work like the halo. */
export function PromoFrame({
  children,
  marked,
  className,
  style,
}: {
  children: ReactNode
  marked?: boolean
  className?: string
  /** For a per-instance animation delay a ladder staggers its rungs with; nothing else. */
  style?: CSSProperties
}) {
  const rootRef = useRef<HTMLDivElement>(null)

  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const root = rootRef.current
    if (!root) return
    const rect = root.getBoundingClientRect()
    const x = event.clientX - rect.left
    const y = event.clientY - rect.top
    root.style.setProperty('--spot-x', `${x}px`)
    root.style.setProperty('--spot-y', `${y}px`)
    if (event.pointerType === 'touch' || prefersReducedMotion() || !rect.width || !rect.height) {
      return
    }
    // The pointer's offset from the card's centre, -0.5 … 0.5 on each axis. The card leans
    // TOWARD the pointer: a pointer at the top edge tips the top away from the viewer.
    const dx = x / rect.width - 0.5
    const dy = y / rect.height - 0.5
    const tiltX = (-dy * PROMO_TILT_MAX_DEG).toFixed(2)
    const tiltY = (dx * PROMO_TILT_MAX_DEG).toFixed(2)
    root.style.transform = `perspective(${PROMO_TILT_PERSPECTIVE_PX}px) rotateX(${tiltX}deg) rotateY(${tiltY}deg)`
  }

  const onPointerLeave = () => {
    rootRef.current?.style.removeProperty('transform')
  }

  return (
    <div
      ref={rootRef}
      onPointerMove={onPointerMove}
      onPointerLeave={onPointerLeave}
      className={twMerge(
        // `isolate` so the halo's negative z-index stays inside this frame — without its own
        // stacking context the layer would sink behind the page and vanish. `group` is what
        // lets the spotlight inside fade in on the frame's hover.
        'group duration-base ease-standard relative isolate h-full rounded-lg transition-transform hover:-translate-y-1',
        marked && 'shadow-md',
        className,
      )}
      style={style}
    >
      {marked && (
        <div
          aria-hidden="true"
          data-promo-glow=""
          className="bg-promo-stroke animate-promo-glow absolute -inset-1 -z-10 rounded-xl blur-lg"
        />
      )}
      {/* The stroke IS this box's background; its padding is the stroke's width. The content sits
          opaque on top, so the gradient shows only in that padding and every figure inside keeps
          the surface pair it was audited against (THEME-7). */}
      <div
        data-promo-stroke=""
        className={clsx('bg-promo-stroke relative h-full rounded-lg', marked ? 'p-0.5' : 'p-px')}
      >
        {/* `isolate` here too: the spotlight sits at a negative z-index so it paints ABOVE this
            box's surface and BELOW its text, which only holds inside a stacking context this box
            itself roots. */}
        <div className="bg-surface-raised relative isolate h-full rounded-lg p-4">
          <div
            aria-hidden="true"
            data-promo-spot=""
            className="bg-promo-spotlight duration-base ease-standard pointer-events-none absolute inset-0 -z-10 rounded-lg opacity-0 transition-opacity group-hover:opacity-100"
          />
          {children}
        </div>
      </div>
    </div>
  )
}
