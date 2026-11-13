import { forwardRef, type ComponentPropsWithoutRef } from 'react'
import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'
import { cardStyles } from '../card/cardStyles'

export interface SurfaceButtonProps extends ComponentPropsWithoutRef<'button'> {
  presentation: 'card' | 'row' | 'content'
  padding?: 'none' | 'sm' | 'md' | 'lg'
}

/** An actionable content surface, retaining the caller's content and native button semantics. */
export const SurfaceButton = forwardRef<HTMLButtonElement, SurfaceButtonProps>(
  function SurfaceButton(
    { presentation, padding = 'md', className, type = 'button', ...props },
    ref,
  ) {
    const base = 'min-h-11 min-w-11 text-left disabled:opacity-50 aria-disabled:opacity-50'
    const interactive =
      !props.disabled && props['aria-disabled'] !== true && props['aria-disabled'] !== 'true'
    const styles =
      presentation === 'card'
        ? cardStyles({ padding, interactive, className: clsx(base, className) })
        : clsx(
            base,
            'rounded-md',
            interactive && 'hover:bg-row-bg-hover active:bg-row-bg-active',
            presentation === 'row' && 'flex w-full items-center gap-3 px-4 py-3',
            className,
          )
    return <button ref={ref} type={type} className={twMerge(styles)} {...props} />
  },
)
