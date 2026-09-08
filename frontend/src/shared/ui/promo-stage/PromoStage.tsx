import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { twMerge } from 'tailwind-merge'
import { startAurora } from './aurora'

/** The ground a promotional surface stands on: a recessed panel with an aurora moving behind
 *  whatever it holds, under a film of grain (THEME-37).
 *
 *  The aurora is a fragment shader on a canvas — three layers of drifting noise tinted with the
 *  promotional hues, folded into the panel's own recessed colour, all four read from the
 *  stylesheet so the picture re-skins with the theme. Where WebGL is not to be had the same three
 *  hues fall back to blurred CSS blobs drifting on `transform`, so the stage is never blank: the
 *  canvas fades in over the blobs only once the shader has actually started.
 *
 *  The grain is an SVG turbulence filter blended over the whole panel: a static texture that
 *  keeps a smooth gradient from reading as a flat screen fill and costs one raster.
 *
 *  Every layer is clipped to the panel and sits behind the children; the reduced-motion rule
 *  holds the blobs at their first frame and the shader draws one still frame, so what remains is
 *  a still aurora rather than a blank panel. It is one of the three promotional primitives and,
 *  like `PromoFrame`, it is named for the exemption it carries: a reviewer who sees `PromoStage`
 *  under an ordinary list has found the exception spreading, not a new pattern. */
export function PromoStage({ children, className }: { children: ReactNode; className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [live, setLive] = useState(false)
  const grainId = useId()

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const aurora = startAurora(canvas)
    // Announcing the shader from a frame rather than from the effect body keeps the first paint
    // the fallback's, so the fade-in is a fade-in and not a flash.
    const reveal = aurora ? requestAnimationFrame(() => setLive(true)) : 0
    return () => {
      if (reveal) cancelAnimationFrame(reveal)
      aurora?.stop()
    }
  }, [])

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
        <canvas
          ref={canvasRef}
          data-promo-shader=""
          data-live={live ? 'true' : 'false'}
          className={twMerge(
            'duration-slow ease-standard absolute inset-0 size-full transition-opacity',
            live ? 'opacity-100' : 'opacity-0',
          )}
        />
        <svg
          data-promo-grain=""
          className="absolute inset-0 size-full opacity-20 mix-blend-overlay"
          xmlns="http://www.w3.org/2000/svg"
        >
          <filter id={grainId}>
            <feTurbulence
              type="fractalNoise"
              baseFrequency="0.9"
              numOctaves="2"
              stitchTiles="stitch"
            />
            <feColorMatrix type="saturate" values="0" />
          </filter>
          <rect width="100%" height="100%" filter={`url(#${grainId})`} />
        </svg>
      </div>
      {children}
    </div>
  )
}
