import { forwardRef, useRef, type HTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

interface SelectableTextProps extends Omit<
  HTMLAttributes<HTMLSpanElement>,
  'role' | 'tabIndex' | 'onClick' | 'onKeyDown' | 'onPointerDown'
> {
  onActivate: () => void
}

function selectionTouches(element: HTMLElement): boolean {
  const selection = element.ownerDocument.getSelection()
  if (!selection || selection.isCollapsed) return false
  for (let index = 0; index < selection.rangeCount; index += 1) {
    if (selection.getRangeAt(index).intersectsNode(element)) return true
  }
  return false
}

/** An inline text action whose words remain ordinary selectable text. Dragging or long-pressing
 *  to copy never activates it; an intentional tap, Enter or Space does. Inherited typography
 *  and the global focus ring preserve the surrounding prose and its keyboard affordance. */
export const SelectableText = forwardRef<HTMLSpanElement, SelectableTextProps>(
  function SelectableText({ className, onActivate, ...props }, ref) {
    const preserveSelection = useRef(false)
    return (
      <span
        {...props}
        ref={ref}
        role="button"
        tabIndex={0}
        className={twMerge('cursor-pointer rounded-sm select-text active:opacity-80', className)}
        onPointerDown={(event) => {
          preserveSelection.current = selectionTouches(event.currentTarget)
          // A browser collapses a prior selection on pointerdown, before click can inspect it.
          // Preserve a copy selection through that default and suppress this entire press.
          if (preserveSelection.current) event.preventDefault()
        }}
        onClick={(event) => {
          const copying = preserveSelection.current || selectionTouches(event.currentTarget)
          preserveSelection.current = false
          if (copying) return
          event.currentTarget.focus({ preventScroll: true })
          onActivate()
        }}
        onKeyDown={(event) => {
          if (event.key !== 'Enter' && event.key !== ' ') return
          event.preventDefault()
          if (!event.repeat) onActivate()
        }}
      />
    )
  },
)
