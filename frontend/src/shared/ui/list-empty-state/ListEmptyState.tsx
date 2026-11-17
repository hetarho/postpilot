import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { Typography } from '../typography/Typography'

/** Guide: an account's list with nothing in it yet — the picture of what will be here, and one
 *  line saying how the first one is made. The way to make it stays the screen's own button; this
 *  only fills the space the list will take. A list narrowed to nothing is a different answer and
 *  does not use this (it names the narrowing instead). */
export function ListEmptyState({
  illustration,
  children,
  className,
}: {
  /** Decorative; the line beside it says everything. */
  illustration?: string
  children: ReactNode
  className?: string
}) {
  return (
    <div className={clsx('flex flex-col items-center gap-4 px-4 py-8 text-center', className)}>
      {illustration && (
        <img
          src={illustration}
          alt=""
          aria-hidden="true"
          className="animate-rise size-28 object-contain drop-shadow-md sm:size-32"
        />
      )}
      <Typography
        variant="body"
        role="status"
        className="text-content-secondary max-w-sm text-pretty"
      >
        {children}
      </Typography>
    </div>
  )
}
