import type { ComponentType, ReactNode } from 'react'
import { twMerge } from '@/shared/lib'
import { typographyStyles } from '../typography/typographyStyles'

/** Guide: an iOS-settings style group — a small heading over ONE raised card whose rows are
 *  separated by hairlines, each row drilling into its own focused screen. */
export function ListGroup({
  heading,
  headingId,
  children,
  footer,
  className,
}: {
  heading?: ReactNode
  /** The heading's id, for a `section`/`nav` that names itself by it. */
  headingId?: string
  /** The rows: `li` elements (or any block children) carrying `listRowStyles`. */
  children: ReactNode
  footer?: ReactNode
  className?: string
}) {
  return (
    <div className={twMerge('min-w-0', className)}>
      {heading && (
        <h2
          id={headingId}
          className={typographyStyles({ variant: 'groupLabel', className: 'mb-2 px-4' })}
        >
          {heading}
        </h2>
      )}
      <ul className="bg-card-bg divide-divider divide-y overflow-hidden rounded-xl shadow-sm">
        {children}
      </ul>
      {footer && <div className="text-content-tertiary text-meta mt-2 px-4">{footer}</div>}
    </div>
  )
}

/** The glyph chip a list row leads with: the accent's soft tint behind the icon. `lg` is the tile a
 *  big choice card (만들기, 내 스타일, setup) leads with. */
export function ListRowIcon({
  icon: Icon,
  size = 'md',
}: {
  icon: ComponentType<{ className?: string }>
  size?: 'md' | 'lg'
}) {
  return (
    <span
      aria-hidden="true"
      className={
        size === 'lg'
          ? 'bg-badge-accent-bg text-badge-accent-fg flex size-14 shrink-0 items-center justify-center rounded-lg'
          : 'bg-badge-accent-bg text-badge-accent-fg flex size-9 shrink-0 items-center justify-center rounded-[0.75rem]'
      }
    >
      <Icon className={size === 'lg' ? 'size-7' : 'size-5'} />
    </span>
  )
}
