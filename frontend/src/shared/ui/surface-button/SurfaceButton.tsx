import { forwardRef, type ComponentPropsWithoutRef } from 'react'
import { clsx } from 'clsx'
import { cardStyles } from '../card/cardStyles'

export interface SurfaceButtonProps extends ComponentPropsWithoutRef<'button'> {
  presentation: 'card' | 'row' | 'content'
}

/** An actionable content surface, retaining the caller's content and native button semantics. */
export const SurfaceButton = forwardRef<HTMLButtonElement, SurfaceButtonProps>(
  function SurfaceButton({ presentation, className, type = 'button', ...props }, ref) {
    const base = 'min-h-11 min-w-11 text-left disabled:opacity-50 aria-disabled:opacity-50'
    const interactive =
      !props.disabled && props['aria-disabled'] !== true && props['aria-disabled'] !== 'true'
    const styles =
      presentation === 'card'
        ? cardStyles({ interactive, className: clsx(base, className) })
        : clsx(
            base,
            'rounded-md',
            interactive && 'hover:bg-row-bg-hover active:bg-row-bg-active',
            presentation === 'row' && 'w-full px-4 py-3',
            className,
          )
    return <button ref={ref} type={type} className={styles} {...props} />
  },
)
