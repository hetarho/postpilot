import { Children, useEffect, useLayoutEffect, useRef, type ReactNode } from 'react'
import { prefersReducedMotion, twMerge } from '@/shared/lib'

/** How much of a slide must be on screen before it counts as the one the snap settled on. */
const SETTLED_RATIO = 0.6

/** Guide: one slide per view in a horizontal snap strip (THEME-25's strip exception) — a swipe or
 *  a flick parks a whole slide. The caller owns which slide is current and draws its own position
 *  and previous/next controls beside it, since a swipe alone is unreachable with a mouse.
 *
 *  Controlled both ways: a swipe that settles on another slide reports it through
 *  `onIndexChange`, and an `index` the caller changes (a button, a menu) scrolls the strip there.
 *  Every slide stays mounted, so moving to one never waits on it to render. */
export function Carousel({
  index,
  onIndexChange,
  label,
  className,
  slideClassName,
  children,
}: {
  index: number
  onIndexChange: (index: number) => void
  /** Names the strip for assistive technology. */
  label?: string
  className?: string
  slideClassName?: string
  /** One child per slide, keyed. */
  children: ReactNode
}) {
  const stripRef = useRef<HTMLDivElement>(null)
  // The slide the strip is at or travelling to: an index equal to it came from the strip itself
  // (a swipe) or has already been scrolled to, so it must not scroll again.
  const settled = useRef(index)
  const report = useRef(onIndexChange)
  useLayoutEffect(() => {
    report.current = onIndexChange
  }, [onIndexChange])
  const count = Children.count(children)

  // An IntersectionObserver rather than a scroll handler: the slide that is mostly on screen is the
  // one the snap settled on, and nothing reads layout per frame.
  useEffect(() => {
    const strip = stripRef.current
    if (!strip || typeof IntersectionObserver === 'undefined') return
    const slides = Array.from(strip.children)
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting && entry.intersectionRatio >= SETTLED_RATIO)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
        if (!visible) return
        const next = slides.indexOf(visible.target)
        if (next < 0 || next === settled.current) return
        settled.current = next
        report.current(next)
      },
      { root: strip, threshold: SETTLED_RATIO },
    )
    for (const slide of slides) observer.observe(slide)
    return () => observer.disconnect()
  }, [count])

  useEffect(() => {
    if (index === settled.current) return
    settled.current = index
    const strip = stripRef.current
    const first = strip?.children[0] as HTMLElement | undefined
    const target = strip?.children[index] as HTMLElement | undefined
    if (!strip || !first || !target) return
    strip.scrollTo?.({
      left: target.offsetLeft - first.offsetLeft,
      behavior: prefersReducedMotion() ? 'auto' : 'smooth',
    })
  }, [index])

  return (
    <div
      ref={stripRef}
      role={label ? 'group' : undefined}
      aria-label={label}
      className={twMerge(
        'flex snap-x snap-mandatory overflow-x-auto overscroll-x-contain',
        className,
      )}
    >
      {Children.map(children, (child) => (
        <div className={twMerge('w-full shrink-0 snap-center', slideClassName)}>{child}</div>
      ))}
    </div>
  )
}
