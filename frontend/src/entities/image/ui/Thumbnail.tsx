import type { ReactNode } from 'react'
import { clsx } from 'clsx'

interface ThumbnailProps {
  /** Where the pixels come from; undefined renders an empty tile (for a photo still
   *  being converted). */
  src: string | undefined
  alt: string
  /** The photo's real pixel size when the caller knows it (`PostImage` carries both). Only
   *  a hint to the browser — the tile's size is fixed by CSS either way. */
  width?: number
  height?: number
  /** Rendered over the image — a status, a button row. */
  children?: ReactNode
  dimmed?: boolean
}

/** One square tile of the photo strip. */
export function Thumbnail({ src, alt, width, height, children, dimmed }: ThumbnailProps) {
  return (
    <figure className="bg-surface-recessed relative size-32 shrink-0 overflow-hidden rounded-lg">
      {src && (
        <img
          src={src}
          alt={alt}
          width={width}
          height={height}
          // `viewUrl` is the full 1024px-long-edge JPEG, and only about two and a half tiles of
          // the strip are on screen at 360px. Without these the editor pulls every photo of the
          // post at full size on mount, over the same cellular link the batch is still uploading
          // on (design-language §8.6).
          loading="lazy"
          decoding="async"
          className={clsx('h-full w-full object-cover', dimmed && 'opacity-40')}
        />
      )}
      {children}
    </figure>
  )
}
