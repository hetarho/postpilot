import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'
import { Typography } from '../typography/Typography'

/** Guide: the place a thing will stand once there is one — its drawing, faded, and what fills it.
 *  `slot` is the dashed outline of one item that is not there yet (the voice in use, the first
 *  template, the first memory), its drawing beside the words; `open` is a whole list with nothing
 *  in it, the drawing centred over one line that points at the screen's creation dock. The drawing
 *  is decorative and passed in, so the primitive knows no domain. */
export function EmptySlot({
  image,
  title,
  titleAs = 'p',
  titleId,
  children,
  variant = 'slot',
  status = false,
  className,
}: {
  image: string
  title?: ReactNode
  /** A heading when the empty slot stands for a whole section that has none yet. */
  titleAs?: 'p' | 'h2' | 'h3'
  titleId?: string
  children?: ReactNode
  variant?: 'slot' | 'open'
  /** The words are the screen's live status (an empty list said aloud when it settles). */
  status?: boolean
  className?: string
}) {
  const open = variant === 'open'
  return (
    <div
      className={twMerge(
        clsx(
          open
            ? 'mt-10 flex flex-col items-center gap-4 text-center'
            : 'border-divider flex items-center gap-4 rounded-2xl border-2 border-dashed p-5 sm:gap-5 sm:p-6',
        ),
        className,
      )}
    >
      <img
        src={image}
        alt=""
        width={open ? 128 : 64}
        height={open ? 128 : 64}
        className={clsx(
          'shrink-0 object-contain',
          open ? 'animate-rise size-28 sm:size-32' : 'size-14 opacity-60 saturate-50 sm:size-16',
        )}
      />
      <div
        role={status ? 'status' : undefined}
        className={clsx('flex min-w-0 flex-col gap-1', open && 'items-center')}
      >
        {title && (
          <Typography variant="fieldTitle" as={titleAs} id={titleId}>
            {title}
          </Typography>
        )}
        {children && (
          <Typography
            variant="body"
            as="div"
            className={clsx('text-content-secondary', open && 'max-w-measure text-balance')}
          >
            {children}
          </Typography>
        )}
      </div>
    </div>
  )
}
