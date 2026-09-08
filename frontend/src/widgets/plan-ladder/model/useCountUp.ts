import { useEffect, useRef, useState } from 'react'
import { prefersReducedMotion } from '@/shared/lib'

/** Easing for a figure that is being counted up to: fast at first, settling at the end, so the
 *  eye reads the destination rather than the journey. */
function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3)
}

/** One frame later. `requestAnimationFrame` where the environment has it, so the count runs with
 *  the paint; a timer of one frame's length where it does not. Either way the effect itself sets
 *  no state — every update lands in a scheduled frame. */
const scheduleFrame: (callback: (now: number) => void) => () => void =
  typeof requestAnimationFrame === 'function' && typeof cancelAnimationFrame === 'function'
    ? (callback) => {
        const frame = requestAnimationFrame(callback)
        return () => cancelAnimationFrame(frame)
      }
    : (callback) => {
        const timer = setTimeout(() => callback(performance.now()), 16)
        return () => clearTimeout(timer)
      }

/** A whole number that counts up (or down) to `target` instead of swapping to it — the plan
 *  ladder's post counts as the reader drags a slider (THEME-37).
 *
 *  The FIRST value is shown at once: a ladder must not open at zero and climb, since a reader
 *  arriving mid-climb would read a figure the grant does not buy. Only later changes animate,
 *  frame by frame, and the run stops the moment the component leaves. Under reduced motion the
 *  count has no duration, so the first frame is already the destination — a still number rather
 *  than a slower one. */
export function useCountUp(target: number, durationMs: number): number {
  const [shown, setShown] = useState(target)
  // Where the previous run ended (or was cut off), so a slider dragged faster than one count
  // finishes starts the next count from the figure actually on screen.
  const fromRef = useRef(target)

  useEffect(() => {
    const from = fromRef.current
    if (from === target) return
    const duration = prefersReducedMotion() ? 0 : durationMs
    const start = performance.now()
    let cancel = () => {}
    const tick = (now: number) => {
      const progress = duration <= 0 ? 1 : Math.min(1, (now - start) / duration)
      const value = Math.round(from + (target - from) * easeOutCubic(progress))
      fromRef.current = value
      setShown(value)
      if (progress < 1) cancel = scheduleFrame(tick)
    }
    cancel = scheduleFrame(tick)
    return () => cancel()
  }, [target, durationMs])

  return shown
}
