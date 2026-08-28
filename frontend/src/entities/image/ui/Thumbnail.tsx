import type { ReactNode } from 'react'
import { clsx } from 'clsx'

interface ThumbnailProps {
  /** Where the pixels come from; undefined renders an empty tile (for a photo still
   *  being converted). */
  src: string | undefined
  alt: string
  /** Rendered over the image — a status, a button row. */
  children?: ReactNode
  dimmed?: boolean
}

/** One square tile of the photo strip. */
export function Thumbnail({ src, alt, children, dimmed }: ThumbnailProps) {
  return (
    <figure className="bg-surface-recessed relative size-32 shrink-0 overflow-hidden rounded-lg">
      {src && (
        <img
          src={src}
          alt={alt}
          className={clsx('h-full w-full object-cover', dimmed && 'opacity-40')}
        />
      )}
      {children}
    </figure>
  )
}
