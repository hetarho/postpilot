import { useLayoutEffect, useRef, useState, type RefObject } from 'react'

/** The attribute every panel this hook places carries, so an ancestor overlay's outside-press
 *  test can treat a press inside it as inside (THEME-30). */
export const ANCHORED_PANEL_ATTRIBUTE = 'data-anchored-panel'

/** How a panel is placed against its trigger: the gap between them, the clear space kept from
 *  the viewport's edges, the shortest the panel is squeezed to, and how much of the viewport it
 *  may take when there is more room than it needs. */
export interface AnchoredPanelPlacement {
  gapPx: number
  gutterPx: number
  minPx: number
  maxViewportRatio: number
}

/** Where the open panel is painted, in viewport coordinates. It is `position: fixed` at the
 *  document body, so these are the measured trigger's own numbers rather than an offset inside
 *  some containing block. */
export interface AnchoredPanelBox {
  left: number
  /** Set only when the panel spans its trigger. */
  width?: number
  /** Set for a panel that drops DOWN — the distance from the viewport's top edge. */
  top?: number
  /** Set for a panel that flipped UP — the distance from the viewport's bottom edge. */
  bottom?: number
  maxHeight: number
  drop: 'down' | 'up'
}

export function samePanelBox(a: AnchoredPanelBox, b: AnchoredPanelBox): boolean {
  return (
    a.left === b.left &&
    a.width === b.width &&
    a.top === b.top &&
    a.bottom === b.bottom &&
    a.maxHeight === b.maxHeight &&
    a.drop === b.drop
  )
}

interface AnchoredPanelOptions {
  open: boolean
  triggerRef: RefObject<HTMLElement | null>
  panelRef: RefObject<HTMLElement | null>
  placement: AnchoredPanelPlacement
  /** `trigger` spans the panel across its trigger (a listbox); `content` keeps the panel's CSS
   *  width and starts it at the trigger's left edge, clamped inside the viewport's gutters. */
  width: 'trigger' | 'content'
  /** The trigger left the viewport, and whether focus was inside the panel at that moment. */
  onGone: (focusWasInside: boolean) => void
  /** Anything else the measurement depends on, such as a listbox's options. */
  remeasure?: unknown
}

/** Places a panel portalled to the document body against the trigger it belongs to (THEME-30).
 *  An `absolute` panel is bounded by every scrolling ancestor, and a field near a sheet's bottom
 *  is exactly the one whose panel flips upward out of it; a portal has no ancestor to be clipped
 *  by, at the cost of tracking the trigger itself.
 *
 *  A trigger can sit anywhere in the viewport, and nothing in CSS knows where — a `dvh` ceiling is
 *  the same number for a field at the top of the page and for one in the docked bar. So the room
 *  is MEASURED: the panel opens downward into the gap it actually has, flips above the trigger
 *  when that gap is too small and the one overhead is larger, and scrolls inside whichever it
 *  took. The ratio caps it where there is more room than it needs; the floor keeps it usable
 *  rather than squeezing it to a sliver for a trigger pinned against an edge.
 *
 *  The measurement is repeated for as long as the panel is open — on a captured `scroll`, since
 *  `scroll` does not bubble and any ancestor may be the one scrolling, and on `resize` — and the
 *  panel is given up once its trigger leaves the viewport (`onGone`). The box is kept BY VALUE:
 *  callers render inline options, so a fresh object each measurement would be a new state on
 *  every render and a new render on every state. */
export function useAnchoredPanel({
  open,
  triggerRef,
  panelRef,
  placement,
  width,
  onGone,
  remeasure,
}: AnchoredPanelOptions): AnchoredPanelBox | undefined {
  const [box, setBox] = useState<AnchoredPanelBox>()
  // Held by its latest value, so a caller's inline callback never re-runs the measurement.
  const gone = useRef(onGone)
  useLayoutEffect(() => {
    gone.current = onGone
  })
  const { gapPx, gutterPx, minPx, maxViewportRatio } = placement
  // A `content` panel's width is only known once it is mounted, which happens after the first
  // box is set; measuring again at that point is what lets its left edge be clamped.
  const mounted = box !== undefined

  useLayoutEffect(() => {
    const trigger = triggerRef.current
    if (!open || !trigger) return
    const measure = () => {
      const anchor = trigger.getBoundingClientRect()
      // Gone from the viewport, which happens when its own scroller carries it away. A panel
      // anchored to nothing is worse than no panel, so it is given up.
      //
      // `height > 0` is what separates "laid out, and scrolled out of sight" from "not laid out
      // at all": scrolling never changes a rect's size, while an environment with no layout
      // engine reports every rect as zero. Without it the panel would refuse to open in jsdom —
      // and in any browser during the frame before layout.
      const isGone =
        anchor.height > 0 &&
        (anchor.bottom <= 0 ||
          anchor.top >= window.innerHeight ||
          anchor.right <= 0 ||
          anchor.left >= window.innerWidth)
      if (isGone) {
        gone.current(panelRef.current?.contains(document.activeElement) ?? false)
        return
      }
      const gap = gapPx + gutterPx
      const below = window.innerHeight - anchor.bottom - gap
      const above = anchor.top - gap
      const flip = below < minPx && above > below
      const room = Math.min(flip ? above : below, window.innerHeight * maxViewportRatio)
      let left = anchor.left
      let spanned: number | undefined
      if (width === 'trigger') {
        spanned = anchor.width
      } else {
        const panelWidth = panelRef.current?.getBoundingClientRect().width ?? 0
        const rightmost = Math.max(gutterPx, window.innerWidth - gutterPx - panelWidth)
        left = Math.min(Math.max(anchor.left, gutterPx), rightmost)
      }
      const next: AnchoredPanelBox = {
        drop: flip ? 'up' : 'down',
        left,
        width: spanned,
        top: flip ? undefined : anchor.bottom + gapPx,
        bottom: flip ? window.innerHeight - anchor.top + gapPx : undefined,
        maxHeight: Math.max(room, minPx),
      }
      setBox((current) => (current && samePanelBox(current, next) ? current : next))
    }
    measure()
    window.addEventListener('scroll', measure, true)
    window.addEventListener('resize', measure)
    return () => {
      window.removeEventListener('scroll', measure, true)
      window.removeEventListener('resize', measure)
    }
  }, [
    open,
    triggerRef,
    panelRef,
    gapPx,
    gutterPx,
    minPx,
    maxViewportRatio,
    width,
    remeasure,
    mounted,
  ])

  return box
}
