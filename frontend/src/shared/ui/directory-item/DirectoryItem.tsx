import type { ReactNode } from 'react'
import { cardStyles } from '../card/cardStyles'

/** A resource's content and separate actions on one shared plane. The caller owns link semantics. */
export function DirectoryItem({
  icon,
  title,
  description,
  status,
  metadata,
  actions,
  interactive = false,
  className,
}: {
  icon?: ReactNode
  title: ReactNode
  description?: ReactNode
  status?: ReactNode
  metadata?: ReactNode
  actions?: ReactNode
  interactive?: boolean
  className?: string
}) {
  return (
    <li
      className={cardStyles({
        padding: 'sm',
        interactive,
        className: `relative flex min-w-0 items-start gap-3 sm:p-5 ${className ?? ''}`,
      })}
    >
      {icon && (
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-12 shrink-0 items-center justify-center rounded-lg"
        >
          {icon}
        </span>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-start gap-2">
          <div className="min-w-0 flex-1">{title}</div>
          {actions && (
            <div className="relative z-10 flex shrink-0 items-center gap-2">{actions}</div>
          )}
        </div>
        {description && (
          <div className="text-content-secondary mt-1 min-w-0 break-words">{description}</div>
        )}
        {status && <div className="mt-2 min-w-0">{status}</div>}
        {metadata && (
          <div className="mt-3 flex min-w-0 flex-wrap items-center gap-2">{metadata}</div>
        )}
      </div>
    </li>
  )
}
