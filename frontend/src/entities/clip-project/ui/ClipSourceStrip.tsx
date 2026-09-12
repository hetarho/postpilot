import { Film } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDuration } from '@/shared/lib/video'
import { Button, Typography } from '@/shared/ui'

interface SourceTile {
  fingerprint: string
  filename: string
  durationMs: number
  previewURL?: string
  status?: string
}

/** The same source selector serves current uploads and retained observations.
 * Thumbnails never play; the selected source's player belongs below the strip. */
export function ClipSourceStrip({
  sources,
  selected,
  onSelect,
  label,
}: {
  sources: readonly SourceTile[]
  selected: string
  onSelect: (fingerprint: string) => void
  label: string
}) {
  const { t } = useTranslation('clips')
  return (
    <div className="min-w-0 space-y-2">
      <ul
        aria-label={label}
        className="-mx-4 flex snap-x snap-mandatory scroll-px-4 gap-2 overflow-x-auto overscroll-x-contain px-4 py-2 sm:mx-0 sm:scroll-px-2 sm:px-2"
      >
        {sources.map((source) => (
          <li key={source.fingerprint} className="w-40 shrink-0 snap-start">
            <Button
              variant={source.fingerprint === selected ? 'secondary' : 'ghost'}
              className="w-full"
              aria-pressed={source.fingerprint === selected}
              aria-label={t('source.inspect', { filename: source.filename })}
              onClick={() => onSelect(source.fingerprint)}
            >
              <span className="flex w-32 min-w-0 flex-col gap-2 py-3 text-left">
                <span className="bg-surface-recessed relative flex aspect-video items-center justify-center overflow-hidden rounded-md">
                  {source.previewURL ? (
                    <video
                      key={source.previewURL}
                      src={source.previewURL}
                      preload="metadata"
                      muted
                      playsInline
                      tabIndex={-1}
                      aria-hidden="true"
                      className="pointer-events-none absolute inset-0 h-full w-full object-cover"
                    />
                  ) : (
                    <Film aria-hidden="true" className="text-content-tertiary size-6" />
                  )}
                </span>
                <Typography variant="label" className="line-clamp-2 break-all">
                  {source.filename}
                </Typography>
                <Typography variant="meta" className="text-content-secondary">
                  {formatDuration(source.durationMs)}
                </Typography>
                {source.status && (
                  <Typography variant="meta" className="break-words">
                    {source.status}
                  </Typography>
                )}
              </span>
            </Button>
          </li>
        ))}
      </ul>
      <Typography variant="meta" className="text-content-secondary block">
        {t('source.position', {
          current: sources.findIndex((source) => source.fingerprint === selected) + 1,
          total: sources.length,
        })}
      </Typography>
    </div>
  )
}
