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
   *  A phone screen has ONE scroller (design-language §4.4): a fixed-`rows` textarea holding more
   *  text than it shows swallows every vertical swipe that lands on it, and on a `w-full` field
   *  the only place left to scroll the page is the 16px gutter. */
  autoGrow?: boolean
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { appearance = 'well', autoGrow = false, className, value, onChange, ...props },
  ref,
) {
  const inner = useRef<HTMLTextAreaElement | null>(null)

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
    if (!node || !autoGrow) return
    // Collapse first: scrollHeight can only ever report >= the current height, so without this the
    // field would grow monotonically and never shrink when text is deleted.
    node.style.height = 'auto'
    node.style.height = `${node.scrollHeight}px`
  }, [autoGrow])

  // Layout effect, not effect: resizing after paint would show one frame at the wrong height on
  // every keystroke. Re-runs on `value` so a programmatic change grows the field too.
  useLayoutEffect(resize, [resize, value])

  // The height is an inline pixel value, so anything that changes how the SAME text wraps — a
  // window resize, an orientation change, or crossing the `sm:` breakpoint where the type steps
  // from 16px to 14px — invalidates it. Without this the field keeps its stale height and, because
  // `autoGrow` also sets `overflow-hidden`, the tail is clipped with no scrollbar to reach it.
  useLayoutEffect(() => {
    const node = inner.current
    if (!node || !autoGrow || typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(resize)
    observer.observe(node)
    return () => observer.disconnect()
  }, [autoGrow, resize])

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
          'text-field-fg placeholder:text-field-placeholder disabled:text-content-disabled min-h-11 w-full resize-none disabled:opacity-50',
          // Nothing to scroll when the box always fits its content — this is what stops the field
          // from competing with the page for the swipe.
          autoGrow && 'overflow-hidden',
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
