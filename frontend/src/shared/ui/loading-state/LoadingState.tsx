import { useEffect, useState } from 'react'
import { twMerge } from '@/shared/lib'
import { cardStyles } from '../card/cardStyles'
import { Spinner } from '../spinner/Spinner'

type LoadingVariant = 'list' | 'cards' | 'form' | 'detail' | 'table' | 'inline'

const DEFAULT_ROWS = 3
const MAX_ROWS = 6

/** Mount the live region empty before adding text. Later updates keep the same node. */
export function LoadingStatus({
  label,
  className = 'sr-only',
}: {
  label: string
  className?: string
}) {
  const [ready, setReady] = useState(false)
  useEffect(() => {
    let frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => setReady(true))
    })
    return () => cancelAnimationFrame(frame)
  }, [])
  return (
    <span role="status" aria-live="polite" aria-atomic="true" className={className}>
      {ready ? label : ''}
    </span>
  )
}

/** A decorative shape; its dimensions are supplied by the composition using it. */
export function Skeleton({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={twMerge('loading-skeleton bg-surface-recessed block rounded-md', className)}
    />
  )
}

function LoadingRow({ variant }: { variant: Exclude<LoadingVariant, 'inline'> }) {
  if (variant === 'cards') {
    return (
      <div className={cardStyles({ className: 'space-y-4' })}>
        <Skeleton className="h-5 w-2/3" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    )
  }
  if (variant === 'form') {
    return (
      <div className="space-y-2">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-12 w-full" />
      </div>
    )
  }
  if (variant === 'detail') {
    return (
      <div className="space-y-3">
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    )
  }
  if (variant === 'table') {
    return (
      <div className="flex min-h-12 items-center gap-4">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-4 w-1/4" />
        <Skeleton className="h-4 w-1/4" />
      </div>
    )
  }
  return (
    <div className={cardStyles({ padding: 'sm', className: 'flex min-h-16 items-center gap-4' })}>
      <Skeleton className="size-10 shrink-0" />
      <div className="min-w-0 flex-1 space-y-2">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-4 w-1/3" />
      </div>
    </div>
  )
}

export function LoadingState({
  variant,
  label,
  className,
  rows = DEFAULT_ROWS,
}: {
  variant: LoadingVariant
  label: string
  className?: string
  rows?: number
}) {
  const count = Number.isFinite(rows)
    ? Math.min(MAX_ROWS, Math.max(1, Math.floor(rows)))
    : DEFAULT_ROWS

  return (
    <div className={className}>
      <LoadingStatus label={label} />
      {variant === 'inline' ? (
        <div
          aria-busy="true"
          className="loading-reveal text-content-tertiary flex items-center gap-2"
        >
          <Spinner />
          <span aria-hidden="true">{label}</span>
        </div>
      ) : (
        <div
          aria-busy="true"
          className={twMerge(
            'loading-reveal grid gap-4',
            variant === 'cards' && 'sm:grid-cols-2 lg:grid-cols-3',
          )}
        >
          {Array.from({ length: count }, (_, index) => (
            <LoadingRow key={index} variant={variant} />
          ))}
        </div>
      )}
    </div>
  )
}
