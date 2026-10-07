import {
  forwardRef,
  useCallback,
  useLayoutEffect,
  useRef,
  type TextareaHTMLAttributes,
} from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  appearance?: 'well' | 'bare'
  /** Grow with the value instead of scrolling inside a fixed box. `rows` becomes the minimum.
   *  A phone screen has ONE scroller (THEME-25): a fixed-`rows` textarea holding more
   *  text than it shows swallows every vertical swipe that lands on it, and on a `w-full` field
   *  the only place left to scroll the page is the 16px gutter. */
  autoGrow?: boolean
  /** Initial writing space from the measured field top to the visible viewport bottom. */
  viewportAllocation?: { reservedBottom?: number }
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  {
    appearance = 'well',
    autoGrow = false,
    viewportAllocation,
    className,
    value,
    onChange,
    ...props
  },
  ref,
) {
  const inner = useRef<HTMLTextAreaElement | null>(null)
  const allocatedMinimum = useRef<string | undefined>(undefined)
  const grow = autoGrow || viewportAllocation !== undefined
  const reservedBottom = viewportAllocation?.reservedBottom ?? 0

  // Callback ref so the primitive can measure while still honouring the caller's ref.
  const setRef = useCallback(
    (node: HTMLTextAreaElement | null) => {
      inner.current = node
      if (typeof ref === 'function') ref(node)
      else if (ref) ref.current = node
    },
    [ref],
  )

  const resize = useCallback(() => {
    const node = inner.current
    if (!node) return
    if (!viewportAllocation && allocatedMinimum.current !== undefined) {
      node.style.minHeight = allocatedMinimum.current
      allocatedMinimum.current = undefined
    }
    if (!grow) return
    // Collapse first: scrollHeight can only ever report >= the current height, so without this the
    // field would grow monotonically and never shrink when text is deleted.
    if (viewportAllocation) {
      if (allocatedMinimum.current === undefined) allocatedMinimum.current = node.style.minHeight
      const computed = getComputedStyle(node)
      const lineHeight =
        Number.parseFloat(computed.lineHeight) || Number.parseFloat(computed.fontSize) * 1.2 || 0
      const padding =
        (Number.parseFloat(computed.paddingTop) || 0) +
        (Number.parseFloat(computed.paddingBottom) || 0)
      const rowsFloor = lineHeight * node.rows + padding
      const viewport = window.visualViewport?.height ?? window.innerHeight
      // Document-relative top keeps ordinary scrolling from growing the field repeatedly.
      const top = Math.max(0, node.getBoundingClientRect().top + window.scrollY)
      node.style.minHeight = `${Math.max(rowsFloor, viewport - top - Math.max(0, reservedBottom))}px`
    }
    node.style.height = 'auto'
    node.style.height = `${node.scrollHeight}px`
    // A caller may cap the growth with `max-h-*` — a long generated styleguide would otherwise put
    // the button that saves it thousands of pixels past the caret (THEME-24). Once the cap clamps the
    // box, the field has to scroll again, so the overflow is decided from the measurement rather
    // than hard-coded: uncapped it stays hidden, capped it becomes the field's own bounded scroller.
    node.style.overflowY = node.scrollHeight > node.clientHeight ? 'auto' : 'hidden'
  }, [grow, reservedBottom, viewportAllocation])

  // Layout effect, not effect: resizing after paint would show one frame at the wrong height on
  // every keystroke. Re-runs on `value` so a programmatic change grows the field too.
  useLayoutEffect(resize, [resize, value])

  // The height is an inline pixel value, so anything that changes how the SAME text wraps — a
  // window resize, an orientation change, or crossing the `sm:` breakpoint where the type steps
  // from 16px to 14px — invalidates it. Without this the field keeps its stale height and, because
  // `autoGrow` also sets `overflow-hidden`, the tail is clipped with no scrollbar to reach it.
  useLayoutEffect(() => {
    const node = inner.current
    if (!node || !grow || typeof ResizeObserver === 'undefined') return
    // Measuring changes this same element's height. Defer observer-triggered
    // writes to the next frame so resizing cannot re-enter the current delivery.
    let frame: number | undefined
    const observer = new ResizeObserver(() => {
      if (frame !== undefined) return
      frame = window.requestAnimationFrame(() => {
        frame = undefined
        resize()
      })
    })
    observer.observe(node)
    return () => {
      observer.disconnect()
      if (frame !== undefined) window.cancelAnimationFrame(frame)
    }
  }, [grow, resize])

  useLayoutEffect(() => {
    if (!viewportAllocation) return
    window.addEventListener('resize', resize)
    window.visualViewport?.addEventListener('resize', resize)
    return () => {
      window.removeEventListener('resize', resize)
      window.visualViewport?.removeEventListener('resize', resize)
    }
  }, [viewportAllocation, resize])

  return (
    <textarea
      ref={setRef}
      value={value}
      onChange={(event) => {
        onChange?.(event)
        resize()
      }}
      className={twMerge(
        clsx(
          'text-field-fg placeholder:text-field-placeholder disabled:text-content-disabled min-h-10 w-full resize-none disabled:opacity-50 pointer-coarse:min-h-11',
          // `resize()` owns overflow-y from here on (see above); this is only the pre-measurement
          // state, so the first paint never flashes a scrollbar.
          grow && 'overflow-hidden',
          appearance === 'well'
            ? 'bg-field-bg hover:bg-field-bg-hover focus:bg-field-bg-focus rounded-md px-4 py-2 text-base sm:text-sm'
            : 'bg-transparent',
          className,
        ),
      )}
      {...props}
    />
  )
})
