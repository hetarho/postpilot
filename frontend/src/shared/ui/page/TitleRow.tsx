import type { ReactNode } from 'react'
import { twMerge } from '@/shared/lib'
import { ContextualReturn } from '../navigation/ContextualReturn'
import { useReturnClaim } from '../navigation/useReturnClaim'
import { typographyStyles, type TypographyVariant } from '../typography/typographyStyles'

/** A screen's title with its way back at the start of the same row (THEME-75): the ← stands
 *  before the title, centred on the title's first line, and the title wraps in its own column
 *  beside it, so a long one never runs under the arrow and the arrow never takes a row of its own.
 *
 *  The route's return is claimed from the frame or the shell that would otherwise draw it above
 *  the page. A screen whose way back is its own fixed step (checkout → plans) passes it as `back`.
 *  With neither, the title stands alone, exactly as it would without this row. */
export function TitleRow({
  variant = 'largeTitle',
  back,
  className,
  children,
}: {
  /** The title's type role, so the ← centres on its first line at every size. */
  variant?: TypographyVariant
  /** The screen's own way back, in place of the route's. */
  back?: ReactNode
  className?: string
  children: ReactNode
}) {
  const routeReturn = useReturnClaim(!back)
  const control = back ?? (routeReturn ? <ContextualReturn /> : null)
  if (!control) return <div className={twMerge('min-w-0', className)}>{children}</div>
  return (
    <div className={twMerge('flex min-w-0 items-start gap-3', className)}>
      {/* One line of the title's own height: the control centres on it, whatever the title's
          size and however many lines it wraps to. */}
      <span className={typographyStyles({ variant, className: 'flex h-lh shrink-0 items-center' })}>
        {control}
      </span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  )
}
