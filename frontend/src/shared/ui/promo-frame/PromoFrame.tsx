import { useRef, type CSSProperties, type PointerEvent, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** A promotional frame: content on its own surface, inside a still stroke of accent gradient,
 *  with a spotlight that follows a fine pointer across it.
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
 *  The SPOTLIGHT is a soft accent disc centred on the pointer, faded in under `hover:` — which
 *  Tailwind compiles to `(hover: hover)`, so a touchscreen never pays for it. The pointer position
 *  is written straight into two custom properties on the frame rather than into React state: a
 *  pointer moves at 60–120 Hz and a render per event would make the card the slowest thing on the
 *  page for no visible gain.
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
    root.style.setProperty('--spot-x', `${event.clientX - rect.left}px`)
    root.style.setProperty('--spot-y', `${event.clientY - rect.top}px`)
  }

  return (
    <div
      ref={rootRef}
      onPointerMove={onPointerMove}
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
