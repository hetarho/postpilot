import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { Play } from 'lucide-react'
import { formatDuration } from '@/shared/lib'
import { typographyStyles } from '@/shared/ui'

interface VideoTileProps {
  /** Where the clip comes from: the presigned view URL, or a local object URL for one this
   *  client just uploaded. Undefined renders an empty tile. */
  src: string | undefined
  durationMs: number
  /** The container's type, so the platform can decide it can play this before fetching. */
  contentType?: string
  /** Rendered over the clip — a status, a button row. */
  children?: ReactNode
  dimmed?: boolean
  onError?: () => void
}

/** One square tile of the strip, for a clip.
 *
 *  `preload="metadata"` and no autoplay: the strip must not start pulling 200 MB of video the
 *  moment the editor mounts, and a clip that played itself under the thumb would be worse than
 *  one that waits to be tapped (VIDEO-14). The duration badge is what tells a video tile from a
 *  photo tile at a glance, so it is text and not only the play glyph.
 */
export function VideoTile({
  src,
  durationMs,
  contentType,
  children,
  dimmed,
  onError,
}: VideoTileProps) {
  return (
    <figure className="bg-surface-recessed relative size-32 shrink-0 overflow-hidden rounded-lg">
      {src && (
        <video
          src={src}
          controls
          preload="metadata"
          playsInline
          onError={onError}
          className={clsx('h-full w-full object-cover', dimmed && 'opacity-40')}
        >
          {contentType && <source src={src} type={contentType} />}
        </video>
      )}
      <span
        aria-hidden="true"
        className={typographyStyles({
          variant: 'meta',
          className:
            'bg-media-scrim-bg/90 text-media-scrim-fg pointer-events-none absolute right-1 bottom-1 flex items-center gap-1 rounded px-1.5 py-0.5 tabular-nums',
        })}
      >
        <Play className="size-3" />
        {formatDuration(durationMs)}
      </span>
      {children}
    </figure>
  )
}
