import { useEffect, useLayoutEffect, useRef, useState } from 'react'

/** How far below the screen an element counts as near: half a screen. A list that asks for its
 *  next page only once its end is visible makes the reader wait at the end of every page; a whole
 *  screen measured too far — twenty rows end inside it on a 390px phone and on a 1280px desk, so
 *  the second page was fetched on arrival for a reader who had not scrolled at all. */
const NEAR_VIEWPORT_MARGIN = '0px 0px 50% 0px'

/** Calls `onNear` while the element this returns a ref for is within half a screen of the viewport, and
 *  `enabled`. Re-enabling it while the element is still near calls it again, which is how a list
 *  keeps loading until its end is a screen away (POST-90).
 *
 *  Where `IntersectionObserver` does not exist (jsdom) it never fires, so nothing loads on its own. */
export function useNearViewport<T extends Element>(onNear: () => void, enabled: boolean) {
  const [node, setNode] = useState<T | null>(null)
  const latest = useRef(onNear)
  useLayoutEffect(() => {
    latest.current = onNear
  })

  useEffect(() => {
    if (!node || !enabled || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) latest.current()
      },
      { rootMargin: NEAR_VIEWPORT_MARGIN },
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [node, enabled])

  return setNode
}
